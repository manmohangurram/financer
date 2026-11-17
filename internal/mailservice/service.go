// Package mailservice links IMAP mailboxes, polls them, learns parsing
// templates with the user's LLM, and turns reviewed extractions into
// transactions.
package mailservice

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/mohan9182/financer/internal/accountservice"
	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/llmsettings"
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
	llm      *llmsettings.Service

	throttleMu sync.Mutex
	lastPoll   map[string]time.Time
	lastLearn  map[string]time.Time
}

func New(r repo.MailRepo, secret *string, txns *transactionservice.Service, accounts *accountservice.Service, llm *llmsettings.Service) *Service {
	return &Service{
		repo: r, secret: secret, txns: txns, accounts: accounts, llm: llm,
		lastPoll: map[string]time.Time{}, lastLearn: map[string]time.Time{},
	}
}

// --- mailboxes ---

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
	if s.secret == nil {
		return nil, errors.Internal("FINANCER_SECRET_KEY is not set; cannot store mailbox credentials")
	}
	secretEnc, err := mail.Encrypt(*s.secret, req.Password)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.CreateAccount(userID, &repo.NewMailAccount{
		Label: strings.TrimSpace(req.Label), IMAPHost: host, IMAPPort: req.IMAPPort,
		Username: strings.TrimSpace(req.Username), SecretEnc: secretEnc,
	})
	if err != nil {
		return nil, err
	}
	return accountWire(row), nil
}

// Update edits a mailbox. An empty password keeps the stored secret.
func (s *Service) Update(userID, id string, req model.ReqUpdateMailAccount) (*model.MailAccount, error) {
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
	if req.Password != "" && s.secret == nil {
		return nil, errors.Internal("FINANCER_SECRET_KEY is not set; cannot store mailbox credentials")
	}
	enc := ""
	if req.Password != "" {
		if enc, err = mail.Encrypt(*s.secret, req.Password); err != nil {
			return nil, err
		}
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	ok, err := s.repo.UpdateAccount(userID, id, strings.TrimSpace(req.Label), host, req.IMAPPort,
		strings.TrimSpace(req.Username), enc, enabled)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.NotFound("mail account " + id + " not found")
	}
	rows, err := s.repo.ListAccounts(userID)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.ID == id {
			return accountWire(row), nil
		}
	}
	return nil, errors.NotFound("mail account " + id + " not found")
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

// --- messages ---

// Messages returns one page of the queue plus the filtered total and the cursor
// for the next page. Status may be a single status, "all", or "review"
// (pending + unparsed).
func (s *Service) Messages(userID string, status *string, accountID, search, pageToken string, limit int64) ([]*model.MailMessage, int64, string, error) {
	var statuses []repo.MailStatus
	raw := ""
	if status != nil {
		raw = strings.ToLower(strings.TrimSpace(*status))
	}
	switch raw {
	case "", "all", "review":
		if raw == "review" {
			statuses = []repo.MailStatus{repo.MailStatusPending, repo.MailStatusUnparsed}
		}
	case "auto":
		statuses = []repo.MailStatus{repo.MailStatusAuto, repo.MailStatusRejected}
	default:
		normalized := repo.MailStatus(strings.ToUpper(raw))
		switch normalized {
		case repo.MailStatusPending, repo.MailStatusUnparsed, repo.MailStatusIgnored,
			repo.MailStatusAuto, repo.MailStatusRejected:
			statuses = []repo.MailStatus{normalized}
		default:
			return nil, 0, "", errors.BadRequest("unknown status " + fmt.Sprintf("%q", *status))
		}
	}
	limit = clampLimit(limit)
	cursor, err := decodeMailCursor(pageToken)
	if err != nil {
		return nil, 0, "", err
	}
	total, err := s.repo.CountMessagesFiltered(userID, statuses, accountID, search)
	if err != nil {
		return nil, 0, "", err
	}
	rows, err := s.repo.ListMessagesFiltered(userID, statuses, accountID, search, limit+1, cursor)
	if err != nil {
		return nil, 0, "", err
	}
	next := ""
	if int64(len(rows)) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next = encodeMailCursor(last.RowID)
	}
	out := make([]*model.MailMessage, 0, len(rows))
	for _, row := range rows {
		m := &model.MailMessage{
			ID: row.ID, MessageID: row.MessageID, MailAccountID: row.MailAccountID,
			FromAddr: row.FromAddr, FromName: row.FromName,
			Subject: row.Subject, ReceivedAt: row.ReceivedAt,
			Status: string(row.Status), ParsedJSON: row.ParsedJSON,
			Confidence: row.Confidence, BodyPreview: bodyPreview(row.BodyPreview),
			TransactionID: row.TransactionID,
		}
		out = append(out, m)
	}
	return out, total, next, nil
}

func encodeMailCursor(rowID int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(rowID, 10)))
}

func decodeMailCursor(token string) (*repo.MailCursor, error) {
	if token == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, errors.BadRequest("invalid pageToken")
	}
	rowID, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil || rowID <= 0 {
		return nil, errors.BadRequest("invalid pageToken")
	}
	return &repo.MailCursor{RowID: rowID}, nil
}

// Get returns one message with its display body and raw source. Rows stored
// before the parser existed are parsed on demand.
func (s *Service) Get(userID, id string) (*model.MailMessage, error) {
	row, err := s.repo.GetMessage(userID, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errors.NotFound("message " + id + " not found")
	}
	body := ""
	if row.BodyText != nil {
		body = *row.BodyText
	}
	fromName := row.FromName
	if strings.TrimSpace(body) == "" && row.RawText != nil {
		parsed := mail.ParseEmail(*row.RawText)
		body = parsed.Body
		if fromName == "" {
			fromName = parsed.FromName
		}
	}
	msg := &model.MailMessage{
		ID: row.ID, MessageID: row.MessageID, MailAccountID: row.MailAccountID,
		FromAddr: row.FromAddr, FromName: fromName,
		Subject: row.Subject, ReceivedAt: row.ReceivedAt,
		Status: string(row.Status), ParsedJSON: row.ParsedJSON,
		Confidence: row.Confidence, BodyPreview: bodyPreview(body),
		BodyText: &body, RawText: row.RawText, TransactionID: row.TransactionID,
	}
	return msg, nil
}

// bodyPreview is the first non-empty line of the parsed body, bounded so queue
// rows stay one line.
func bodyPreview(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len([]rune(line)) > 140 {
			return string([]rune(line)[:140]) + "…"
		}
		return line
	}
	return ""
}

func clampLimit(limit int64) int64 {
	if limit < 1 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

// Act approves or rejects a batch. Approve items may override any extracted
// field; the effective values are stored in approved_json as training evidence.
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

	extraction, err := effectiveExtraction(msg, item)
	if err != nil {
		return err
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

	approved, err := json.Marshal(extraction)
	if err != nil {
		return errors.Internal("could not store the approved extraction")
	}
	approvedStr := string(approved)
	_, err = s.repo.ResolveMessage(userID, item.ID, repo.MailStatusAuto, &approvedStr, &txnID)
	return err
}

// effectiveExtraction merges the stored extraction with manual overrides.
func effectiveExtraction(msg *repo.MailMessageRow, item model.MailActionItem) (*mail.Extraction, error) {
	var extraction mail.Extraction
	if msg.ParsedJSON != nil {
		if err := json.Unmarshal([]byte(*msg.ParsedJSON), &extraction); err != nil {
			return nil, errors.Internal("stored extraction is not readable")
		}
	}
	if item.Name != nil && strings.TrimSpace(*item.Name) != "" {
		extraction.Merchant = strings.TrimSpace(*item.Name)
	}
	if item.Amount != nil {
		if *item.Amount <= 0 {
			return nil, errors.BadRequest("amount must be greater than zero")
		}
		extraction.Amount = *item.Amount
	}
	if item.Type != nil {
		typ := strings.ToUpper(*item.Type)
		if typ != string(model.TransactionTypeDebit) && typ != string(model.TransactionTypeCredit) {
			return nil, errors.BadRequest("unknown transaction type " + *item.Type)
		}
		extraction.Direction = typ
	}
	if item.OccurredAt != nil {
		if stored := occurredAtStored(item.OccurredAt); stored != "" {
			date := strings.Fields(stored)[0]
			extraction.OccurredAt = &date
		}
	}
	if extraction.Merchant == "" {
		extraction.Merchant = "Unknown"
	}
	if extraction.Amount <= 0 {
		return nil, errors.BadRequest("amount is required")
	}
	if extraction.Direction != string(model.TransactionTypeDebit) && extraction.Direction != string(model.TransactionTypeCredit) {
		return nil, errors.BadRequest("type is required")
	}
	return &extraction, nil
}

// --- polling ---

// PollNow polls one mailbox synchronously (explicit per-mailbox action).
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
	summary, err := s.pollAccount(context.Background(), account)
	if err != nil {
		return nil, err
	}
	s.autoCreate(userID)
	return summary, nil
}

// PollUserAsync starts an asynchronous poll for every enabled mailbox of the
// user and returns whether it started or was throttled.
func (s *Service) PollUserAsync(userID string, force bool) string {
	settings := s.settings(userID)
	throttle := time.Duration(settings.AutoPollThrottleMinutes) * time.Minute
	if throttle <= 0 {
		throttle = 5 * time.Minute
	}
	s.throttleMu.Lock()
	last := s.lastPoll[userID]
	if !force && !last.IsZero() && time.Since(last) < throttle {
		s.throttleMu.Unlock()
		return "throttled"
	}
	s.lastPoll[userID] = time.Now()
	s.throttleMu.Unlock()

	go func() {
		n, err := s.pollUser(userID)
		if err != nil {
			slog.Warn("auto poll failed", "user", userID, "err", err)
			return
		}
		if n > 0 {
			slog.Info("auto poll complete", "user", userID, "mailboxes", n)
		}
		s.autoCreate(userID)
	}()
	return "started"
}

func (s *Service) pollUser(userID string) (int, error) {
	accounts, err := s.repo.ListAccounts(userID)
	if err != nil {
		return 0, err
	}
	polled := 0
	for _, account := range accounts {
		if !account.Enabled {
			continue
		}
		if _, err := s.pollAccount(context.Background(), account); err != nil {
			slog.Warn("mail poll failed", "account", account.ID, "user", userID, "err", err)
			if markErr := s.repo.MarkPolled(account.ID, err.Error()); markErr != nil {
				slog.Warn("could not record poll failure", "account", account.ID, "err", markErr)
			}
			continue
		}
		if err := s.repo.MarkPolled(account.ID, ""); err != nil {
			slog.Warn("could not record poll success", "account", account.ID, "err", err)
		}
		polled++
	}
	return polled, nil
}

func (s *Service) pollAccount(ctx context.Context, account *repo.MailAccountRow) (*mail.PollSummary, error) {
	if s.secret == nil {
		return nil, errors.BadRequest("FINANCER_SECRET_KEY is not set")
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
	settings := s.settings(account.UserID)
	summary, err := mail.PollAccount(ctx, source, s.repo, account, s.generateTemplates, settings.MinSamples)
	if err != nil {
		return nil, err
	}
	slog.Info("mail poll complete", "account", account.ID, "user", account.UserID,
		"matched", summary.Matched, "stored", summary.Stored, "skipped", summary.Skipped)
	return summary, nil
}

// generateTemplates is the LLM hook used by the poll. It enforces the user's
// per-sender cooldown before calling LiteLLM.
func (s *Service) generateTemplates(ctx context.Context, userID, sender string, samples []mail.LLMSample) ([]mail.TemplateSpec, error) {
	if s.llm == nil {
		return nil, nil
	}
	cfg, err := s.llm.Config(userID)
	if err != nil {
		slog.Warn("LLM config unavailable", "user", userID, "err", err)
		return nil, nil
	}
	if cfg == nil {
		return nil, nil
	}
	settings := s.settings(userID)
	cooldown := time.Duration(settings.CooldownMinutes) * time.Minute
	if cooldown <= 0 {
		cooldown = time.Hour
	}
	key := userID + "|" + sender
	s.throttleMu.Lock()
	if last := s.lastLearn[key]; !last.IsZero() && time.Since(last) < cooldown {
		s.throttleMu.Unlock()
		return nil, nil
	}
	s.lastLearn[key] = time.Now()
	s.throttleMu.Unlock()

	start := time.Now()
	specs, err := mail.GenerateTemplates(ctx, *cfg, samples)
	if err != nil {
		slog.Warn("template generation failed", "user", userID, "sender", sender,
			"duration_ms", time.Since(start).Milliseconds(), "err", err)
		return nil, err
	}
	slog.Info("template generated", "user", userID, "sender", sender,
		"templates", len(specs), "duration_ms", time.Since(start).Milliseconds())
	return specs, nil
}

func (s *Service) settings(userID string) *repo.LLMSettingsRow {
	defaults := &repo.LLMSettingsRow{
		TimeoutSecs: 20, Enabled: true, MinSamples: 3, FailureThreshold: 3,
		CooldownMinutes: 60, AutoPollThrottleMinutes: 5,
	}
	if s.llm != nil {
		if row, err := s.llm.Stored(userID); err == nil && row != nil {
			return row
		}
	}
	return defaults
}

// PollOnce polls every enabled mailbox (background ticker) and sweeps bodies.
func (s *Service) PollOnce() (int, error) {
	accounts, err := s.repo.ListPollableAccounts()
	if err != nil {
		return 0, err
	}
	polled := 0
	for _, account := range accounts {
		if _, err := s.pollAccount(context.Background(), account); err != nil {
			slog.Warn("mail poll failed", "account", account.ID, "user", account.UserID, "err", err)
			if markErr := s.repo.MarkPolled(account.ID, err.Error()); markErr != nil {
				slog.Warn("could not record poll failure", "account", account.ID, "err", markErr)
			}
			continue
		}
		if err := s.repo.MarkPolled(account.ID, ""); err != nil {
			slog.Warn("could not record poll success", "account", account.ID, "err", err)
		}
		s.autoCreate(account.UserID)
		polled++
	}
	if _, err := s.repo.SweepBodies(); err != nil {
		slog.Warn("mail body sweep failed", "err", err)
	}
	return polled, nil
}

// autoCreate approves AUTO rows; failures return to the review queue.
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
		if _, err := s.repo.ResolveMessage(userID, failure.ID, repo.MailStatusPending, nil, nil); err != nil {
			slog.Warn("could not return auto-create row to review", "message", failure.ID, "err", err)
		}
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

// --- rules (unchanged behavior) ---

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

// --- wire/validation helpers ---

func accountWire(row *repo.MailAccountRow) *model.MailAccount {
	return &model.MailAccount{
		ID: row.ID, Label: row.Label, IMAPHost: row.IMAPHost, IMAPPort: row.IMAPPort,
		Username: row.Username, Enabled: row.Enabled,
		LastPolledAt: row.LastPolledAt, LastError: row.LastError,
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

// occurredAtStored normalises an extracted/stored/ISO date to the transaction
// storage layout; empty passes through (the service uses "now").
func occurredAtStored(date *string) string {
	if date == nil || *date == "" {
		return ""
	}
	value := strings.TrimSpace(*date)
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UTC().Format("2006-01-02 15:04:05 -0700 MST")
	}
	if d, err := time.Parse("2006-01-02", value); err == nil {
		return d.UTC().Format("2006-01-02 15:04:05 -0700 MST")
	}
	return value
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
