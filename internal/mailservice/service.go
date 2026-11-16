// Package mailservice links IMAP mailboxes, polls them, and turns fetched bank
// alerts into transactions. The poll is stateless: it reads the allowlisted
// window each tick and relies on Message-ID for idempotency.
package mailservice

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mohan9182/financer/internal/accountservice"
	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/mail"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
	"github.com/mohan9182/financer/internal/transactionservice"
)

type Service struct {
	repo     repo.MailRepo
	secret   *string
	txns     *transactionservice.Service
	accounts *accountservice.Service
}

func New(r repo.MailRepo, secret *string, txns *transactionservice.Service, accounts *accountservice.Service) *Service {
	return &Service{repo: r, secret: secret, txns: txns, accounts: accounts}
}

// Create validates and stores a linked mailbox (credential encrypted).
func (s *Service) Create(userID string, req model.ReqMailAccount) (*model.MailAccount, error) {
	host, err := validateHost(req.IMAPHost)
	if err != nil {
		return nil, err
	}
	if err := validatePort(req.IMAPPort); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Username) == "" {
		return nil, errors.BadRequest("username is required")
	}
	if req.Password == "" {
		return nil, errors.BadRequest("password is required")
	}
	if err := validateAllowlist(req.SenderAllowlist); err != nil {
		return nil, err
	}
	if s.secret == nil {
		return nil, errors.Internal("FINANCER_SECRET_KEY is not set; cannot store mailbox credentials")
	}
	secretEnc, err := mail.Encrypt(*s.secret, req.Password)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.CreateAccount(userID, &repo.NewMailAccount{
		Label:           strings.TrimSpace(req.Label),
		IMAPHost:        host,
		IMAPPort:        req.IMAPPort,
		Username:        strings.TrimSpace(req.Username),
		SecretEnc:       secretEnc,
		SenderAllowlist: req.SenderAllowlist,
	})
	if err != nil {
		return nil, err
	}
	return accountWire(row), nil
}

func (s *Service) List(userID string) ([]*model.MailAccount, error) {
	rows, err := s.repo.ListAccounts(userID)
	if err != nil {
		return nil, err
	}
	out := make([]*model.MailAccount, 0, len(rows))
	for _, row := range rows {
		out = append(out, accountWire(row))
	}
	return out, nil
}

func (s *Service) Delete(userID, id string) error {
	ok, err := s.repo.DeleteAccount(userID, id)
	if err != nil {
		return err
	}
	if !ok {
		return errors.NotFound("mail account " + id + " not found")
	}
	return nil
}

// Messages lists stored mail, optionally filtered by status.
func (s *Service) Messages(userID string, status *string, limit int64) ([]*model.MailMessage, error) {
	var filter *repo.MailStatus
	if status != nil && *status != "" && *status != "all" {
		normalized := repo.MailStatus(strings.ToUpper(*status))
		switch normalized {
		case repo.MailStatusPending, repo.MailStatusUnparsed, repo.MailStatusIgnored,
			repo.MailStatusAuto, repo.MailStatusRejected:
			filter = &normalized
		default:
			return nil, errors.BadRequest("unknown status " + fmt.Sprintf("%q", *status))
		}
	}
	if limit < 1 || limit > 500 {
		limit = clampLimit(limit)
	}
	rows, err := s.repo.ListMessages(userID, filter, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*model.MailMessage, 0, len(rows))
	for _, row := range rows {
		m := &model.MailMessage{
			ID: row.ID, MessageID: row.MessageID, FromAddr: row.FromAddr,
			Subject: row.Subject, ReceivedAt: row.ReceivedAt,
			Status: string(row.Status), ParsedJSON: row.ParsedJSON,
			Confidence: row.Confidence, RawText: row.RawText,
			TransactionID: row.TransactionID,
		}
		out = append(out, m)
	}
	return out, nil
}

func clampLimit(limit int64) int64 {
	if limit < 1 {
		return 1
	}
	if limit > 500 {
		return 500
	}
	return limit
}

// Act approves or rejects a batch, per row, so a failure reports exactly which
// one and why.
func (s *Service) Act(userID string, items []model.MailActionItem) (*model.MailActionOutcome, error) {
	out := &model.MailActionOutcome{Succeeded: []string{}, Failed: []model.MailActionFailure{}}
	for _, item := range items {
		if err := s.actOne(userID, item); err != nil {
			out.Failed = append(out.Failed, model.MailActionFailure{ID: item.ID, Error: err.Error()})
			continue
		}
		out.Succeeded = append(out.Succeeded, item.ID)
	}
	return out, nil
}

func (s *Service) actOne(userID string, item model.MailActionItem) error {
	msg, err := s.repo.GetMessage(userID, item.ID)
	if err != nil {
		return err
	}
	if msg == nil {
		return errors.NotFound("message " + item.ID + " not found")
	}
	// An AUTO row with no transaction yet is the poll's handoff to us.
	actionable := msg.Status == repo.MailStatusPending || msg.Status == repo.MailStatusUnparsed ||
		(msg.Status == repo.MailStatusAuto && msg.TransactionID == nil)
	if !actionable {
		return errors.Conflict("message already decided")
	}

	if item.Action == "REJECT" {
		_, err := s.repo.ResolveMessage(userID, item.ID, repo.MailStatusRejected, nil, nil)
		return err
	}
	if item.Action != "APPROVE" {
		return errors.BadRequest("unknown action " + fmt.Sprintf("%q", item.Action))
	}
	if msg.ParsedJSON == nil {
		return errors.BadRequest("nothing was extracted from this mail")
	}
	extraction, err := parseExtraction(*msg.ParsedJSON)
	if err != nil {
		return errors.Internal("stored extraction is not readable")
	}

	accountID := ""
	if item.AccountID != nil {
		accountID = *item.AccountID
	} else {
		owned, err := s.accounts.List(userID)
		if err != nil {
			return err
		}
		pairs := make([][2]*string, 0, len(owned))
		for _, a := range owned {
			id := a.ID
			pairs = append(pairs, [2]*string{&id, a.EndingNumbers})
		}
		resolved := mail.ResolveAccount(extraction.AccountHint, pairs)
		if resolved == nil {
			return errors.BadRequest("no account matches this mail; pass accountId")
		}
		accountID = *resolved
	}

	txnID := uuid.New().String()
	externalID := "mail:" + msg.MessageID
	result, err := s.txns.Create(userID, []transactionservice.Request{{
		ID:          txnID,
		Name:        extraction.Merchant,
		Amount:      extraction.Amount,
		Type:        extraction.Direction,
		OccurredAt:  occurredAtStored(extraction.OccurredAt),
		AccountID:   accountID,
		CategoryIDs: item.CategoryIDs,
		ExternalID:  &externalID,
	}})
	if err != nil {
		return err
	}
	if !result.Success {
		return errors.BadRequest(result.Message)
	}

	_, err = s.repo.ResolveMessage(userID, item.ID, repo.MailStatusAuto, msg.ParsedJSON, &txnID)
	return err
}

// PollNow polls one mailbox immediately instead of waiting for the ticker.
func (s *Service) PollNow(userID, id string) (*mail.PollSummary, error) {
	accounts, err := s.repo.ListAccounts(userID)
	if err != nil {
		return nil, err
	}
	var account *repo.MailAccountRow
	for _, a := range accounts {
		if a.ID == id {
			account = a
			break
		}
	}
	if account == nil {
		return nil, errors.NotFound("mail account " + id + " not found")
	}
	if s.secret == nil {
		return nil, errors.Internal("FINANCER_SECRET_KEY is not set")
	}
	password, err := mail.Decrypt(*s.secret, account.SecretEnc)
	if err != nil {
		return nil, err
	}
	source, err := mail.ConnectIMAP(account, password)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	summary, err := mail.PollAccount(source, s.repo, account, mail.TemplateExtractor{})
	if err != nil {
		return nil, err
	}
	if err := s.repo.MarkPolled(account.ID, ""); err != nil {
		return nil, err
	}
	return summary, nil
}

func (s *Service) Rules(userID string) ([]*model.MailRule, error) {
	rows, err := s.repo.ListRules(userID)
	if err != nil {
		return nil, err
	}
	out := make([]*model.MailRule, 0, len(rows))
	for _, row := range rows {
		out = append(out, ruleWire(row))
	}
	return out, nil
}

func (s *Service) CreateRule(userID string, req model.ReqMailRule) (*model.MailRule, error) {
	input, err := ruleInput(req)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.CreateRule(userID, input)
	if err != nil {
		return nil, err
	}
	return ruleWire(row), nil
}

func (s *Service) UpdateRule(userID, id string, req model.ReqMailRule) error {
	input, err := ruleInput(req)
	if err != nil {
		return err
	}
	ok, err := s.repo.UpdateRule(userID, id, input)
	if err != nil {
		return err
	}
	if !ok {
		return errors.NotFound("mail rule " + id + " not found")
	}
	return nil
}

func (s *Service) DeleteRule(userID, id string) error {
	ok, err := s.repo.DeleteRule(userID, id)
	if err != nil {
		return err
	}
	if !ok {
		return errors.NotFound("mail rule " + id + " not found")
	}
	return nil
}

// PollOnce polls every enabled mailbox, auto-approving AUTO rows that resolve,
// and sweeps bodies of resolved messages.
func (s *Service) PollOnce() (int, error) {
	accounts, err := s.repo.ListPollableAccounts()
	if err != nil {
		return 0, err
	}
	polled := 0
	for _, account := range accounts {
		if err := s.pollAccount(account); err != nil {
			_ = s.repo.MarkPolled(account.ID, err.Error())
			continue
		}
		_ = s.repo.MarkPolled(account.ID, "")
		s.autoCreate(account.UserID)
		polled++
	}
	_, _ = s.repo.SweepBodies()
	return polled, nil
}

func (s *Service) pollAccount(account *repo.MailAccountRow) error {
	if s.secret == nil {
		return errors.BadRequest("FINANCER_SECRET_KEY is not set")
	}
	password, err := mail.Decrypt(*s.secret, account.SecretEnc)
	if err != nil {
		return err
	}
	source, err := mail.ConnectIMAP(account, password)
	if err != nil {
		return err
	}
	defer source.Close()
	_, err = mail.PollAccount(source, s.repo, account, mail.TemplateExtractor{})
	return err
}

// autoCreate approves rows an AUTO rule marked; anything that cannot be
// resolved goes back to the review queue.
func (s *Service) autoCreate(userID string) {
	status := repo.MailStatusAuto
	rows, err := s.repo.ListMessages(userID, &status, 200)
	if err != nil {
		slog.Warn("auto-create: could not list candidates", "err", err)
		return
	}
	var items []model.MailActionItem
	for _, row := range rows {
		if row.TransactionID == nil {
			items = append(items, model.MailActionItem{ID: row.ID, Action: "APPROVE"})
		}
	}
	if len(items) == 0 {
		return
	}
	outcome, err := s.Act(userID, items)
	if err != nil {
		slog.Warn("auto-create failed", "err", err)
		return
	}
	for _, failure := range outcome.Failed {
		// Back to the queue: an ambiguous account must be reviewed, not retried
		// every tick.
		_, _ = s.repo.ResolveMessage(userID, failure.ID, repo.MailStatusPending, nil, nil)
		slog.Warn("auto-create skipped", "id", failure.ID, "err", failure.Error)
	}
}

// Spawn runs the poll loop until the process exits.
func (s *Service) Spawn(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			if n, err := s.PollOnce(); err != nil {
				slog.Warn("mail poll failed", "err", err)
			} else if n > 0 {
				slog.Info("mail poll", "mailboxes", n)
			}
			<-ticker.C
		}
	}()
}

func accountWire(row *repo.MailAccountRow) *model.MailAccount {
	return &model.MailAccount{
		ID: row.ID, Label: row.Label, IMAPHost: row.IMAPHost, IMAPPort: row.IMAPPort,
		Username: row.Username, SenderAllowlist: row.SenderAllowlist,
		Enabled: row.Enabled, LastPolledAt: row.LastPolledAt, LastError: row.LastError,
	}
}

func ruleWire(row *repo.MailRuleRow) *model.MailRule {
	action := string(row.Action)
	if action == "" {
		action = string(repo.MailRuleActionReview)
	}
	categories := row.CategoryIDs
	if categories == nil {
		categories = []string{}
	}
	return &model.MailRule{
		ID: row.ID, Position: row.Position, Enabled: row.Enabled,
		FromPattern: row.FromPattern, SubjectContains: row.SubjectContains,
		BodyContains: row.BodyContains, Action: action,
		AccountID: row.AccountID, CategoryIDs: categories,
	}
}

func ruleInput(req model.ReqMailRule) (*repo.MailRuleInput, error) {
	from := strings.TrimSpace(req.FromPattern)
	subject := strings.TrimSpace(req.SubjectContains)
	body := strings.TrimSpace(req.BodyContains)
	// A rule with no conditions matches every mail, which would make an AUTO
	// rule ingest the whole mailbox.
	if from == "" && subject == "" && body == "" {
		return nil, errors.BadRequest("a rule needs at least one condition")
	}
	action := repo.MailRuleAction(req.Action)
	switch action {
	case repo.MailRuleActionIgnore, repo.MailRuleActionReview, repo.MailRuleActionAuto:
	default:
		return nil, errors.BadRequest("unknown rule action " + fmt.Sprintf("%q", req.Action))
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	var accountID *string
	if req.AccountID != nil && *req.AccountID != "" {
		accountID = req.AccountID
	}
	return &repo.MailRuleInput{
		Position: req.Position, Enabled: enabled, FromPattern: from,
		SubjectContains: subject, BodyContains: body, Action: action,
		AccountID: accountID, CategoryIDs: req.CategoryIDs,
	}, nil
}

func parseExtraction(raw string) (*mail.Extraction, error) {
	var e mail.Extraction
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// occurredAtStored normalises an extracted YYYY-MM-DD date into the UTC
// storage layout used by transactions.
func occurredAtStored(date *string) string {
	if date == nil || *date == "" {
		return ""
	}
	return *date + " 00:00:00 +0000 UTC"
}

// validateHost refuses link-local and metadata addresses; private ranges are
// allowed (LAN mail servers are a valid use case).
func validateHost(raw string) (string, error) {
	host := strings.ToLower(strings.TrimSpace(raw))
	if host == "" {
		return "", errors.BadRequest("imapHost is required")
	}
	if strings.ContainsAny(host, " \t\r\n/@") {
		return "", errors.BadRequest("imapHost must be a bare hostname")
	}
	if ip := net.ParseIP(host); ip != nil && isBlocked(ip) {
		return "", errors.BadRequest("imapHost resolves to a link-local or metadata address")
	}
	return host, nil
}

func isBlocked(ip net.IP) bool {
	if v4 := ip.To4(); v4 != nil {
		return v4[0] == 169 && v4[1] == 254
	}
	if v6 := ip.To16(); v6 != nil {
		seg := uint16(v6[0])<<8 | uint16(v6[1])
		return seg&0xffc0 == 0xfe80 || seg&0xfe00 == 0xfc00
	}
	return false
}

// validatePort accepts only implicit TLS (993), which is what is implemented.
func validatePort(port int64) error {
	switch port {
	case 993:
		return nil
	case 143:
		return errors.BadRequest("port 143 needs STARTTLS, which is not supported yet")
	default:
		return errors.BadRequest("imapPort must be 993 (implicit TLS)")
	}
}

func validateAllowlist(senders []string) error {
	if len(senders) == 0 {
		return errors.BadRequest("senderAllowlist must name at least one sender")
	}
	for _, sender := range senders {
		s := strings.TrimSpace(sender)
		if s == "" || !strings.Contains(s, "@") || strings.ContainsAny(s, " \t\r\n\"") {
			return errors.BadRequest(fmt.Sprintf("senderAllowlist entry %q is not an email address", sender))
		}
	}
	return nil
}
