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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/mohan9182/financer/internal/accountservice"
	cursorlib "github.com/mohan9182/financer/internal/cursor"
	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/llmsettings"
	"github.com/mohan9182/financer/internal/mail"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/netguard"
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

	aiMu     sync.Mutex
	aiStates map[string]*model.AIParseStatus

	// Re-application is triggered from rule saves, which run in the background;
	// serialising it keeps two concurrent runs from racing over auto-booking.
	reapplyMu sync.Mutex
}

func New(r repo.MailRepo, secret *string, txns *transactionservice.Service, accounts *accountservice.Service, llm *llmsettings.Service) *Service {
	return &Service{
		repo: r, secret: secret, txns: txns, accounts: accounts, llm: llm,
		lastPoll: map[string]time.Time{}, lastLearn: map[string]time.Time{},
		aiStates: map[string]*model.AIParseStatus{},
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

// ListPage returns one cursor page of mailboxes and the next token.
func (s *Service) ListPage(userID string, limit int64, pageToken string) ([]*model.MailAccount, string, error) {
	values, err := cursorlib.Decode(pageToken)
	if err != nil {
		return nil, "", err
	}
	var cur *repo.MailAccountCursor
	if values != nil {
		if len(values) != 2 {
			return nil, "", errors.BadRequest("invalid pageToken")
		}
		cur = &repo.MailAccountCursor{CreatedAt: values[0], ID: values[1]}
	}
	rows, next, err := s.repo.ListAccountsPage(userID, limit, cur)
	if err != nil {
		return nil, "", err
	}
	out := make([]*model.MailAccount, 0, len(rows))
	for _, row := range rows {
		out = append(out, accountWire(row))
	}
	return out, next, nil
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
// DeleteMessages removes specific stored messages, scoped to the user. Booked
// transactions are kept: only the mail disappears.
func (s *Service) DeleteMessages(userID string, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.BadRequest("no message ids given")
	}
	n, err := s.repo.DeleteMessagesForUser(userID, ids)
	if err != nil {
		return 0, err
	}
	slog.Info("mail messages deleted", "user", userID, "requested", len(ids), "deleted", n)
	return n, nil
}

// DeleteAllMessages removes every stored message for the user, whatever its
// status (pending, booked or rejected). Transactions stay.
func (s *Service) DeleteAllMessages(userID string) (int64, error) {
	n, err := s.repo.DeleteAllMessages(userID)
	if err != nil {
		return 0, err
	}
	slog.Info("all mail messages deleted", "user", userID, "deleted", n)
	return n, nil
}

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
	if s.aiRunning(userID) {
		return nil, errors.Conflict("AI parse is running; wait for it to finish")
	}
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
	summary, err := s.pollAccount(context.Background(), account, "poll_now")
	if err != nil {
		return nil, err
	}
	s.ReapplyRules(userID)
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

	trigger := "auto"
	if force {
		trigger = "poll_now"
	}
	label := pollLabel(trigger)
	go func() {
		n, err := s.pollUser(userID, trigger)
		if err != nil {
			slog.Warn(label+" failed", "user", userID, "err", err)
			return
		}
		if n > 0 {
			slog.Info(label+" complete", "user", userID, "mailboxes", n)
		}
		s.autoCreate(userID)
	}()
	return "started"
}

func (s *Service) pollUser(userID, trigger string) (int, error) {
	accounts, err := s.repo.ListAccounts(userID)
	if err != nil {
		return 0, err
	}
	polled := 0
	for _, account := range accounts {
		if !account.Enabled {
			continue
		}
		if _, err := s.pollAccount(context.Background(), account, trigger); err != nil {
			slog.Warn(pollLabel(trigger)+" failed", "account", account.ID, "user", userID, "err", err)
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

func (s *Service) pollAccount(ctx context.Context, account *repo.MailAccountRow, trigger string) (*mail.PollSummary, error) {
	return s.pollAccountMode(ctx, account, false, trigger)
}

// pollLabel gives each trigger its own log line.
func pollLabel(trigger string) string {
	switch trigger {
	case "full_poll":
		return "full poll"
	case "poll_now":
		return "poll now"
	default:
		return "auto poll"
	}
}

// pollAccountMode connects and polls; full scans the whole INBOX.
func (s *Service) pollAccountMode(ctx context.Context, account *repo.MailAccountRow, full bool, trigger string) (*mail.PollSummary, error) {
	if s.secret == nil {
		return nil, errors.BadRequest("FINANCER_SECRET_KEY is not set")
	}
	password, err := mail.Decrypt(*s.secret, account.SecretEnc)
	if err != nil {
		return nil, err
	}
	label := pollLabel(trigger)
	slog.Info(label+" start", "account", account.ID, "user", account.UserID,
		"full", full, "last_uid", account.LastUID)
	source, err := mail.ConnectIMAP(account, password)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	settings := s.settings(account.UserID)
	var summary *mail.PollSummary
	if full {
		summary, err = mail.PollAccountFull(ctx, source, s.repo, account, s.generateTemplates, settings.MinSamples)
	} else {
		summary, err = mail.PollAccount(ctx, source, s.repo, account, s.generateTemplates, settings.MinSamples)
	}
	if err != nil {
		return nil, err
	}
	if summary.MaxUID > 0 {
		if err := s.repo.UpdateLastUID(account.ID, summary.MaxUID); err != nil {
			slog.Warn("could not advance mail cursor", "account", account.ID, "err", err)
		}
	}
	slog.Info(label+" complete", "account", account.ID, "user", account.UserID, "full", full,
		"last_uid", summary.MaxUID, "matched", summary.Matched, "stored", summary.Stored, "skipped", summary.Skipped)
	return summary, nil
}

// CleanupUnmatched deletes stored mail that matches no current rule and never
// produced a transaction or an approval. Runs once at boot.
func (s *Service) CleanupUnmatched() (int, error) {
	users, err := s.repo.DistinctMailUsers()
	if err != nil {
		return 0, err
	}
	total := 0
	for _, userID := range users {
		rules, err := s.repo.ListRules(userID)
		if err != nil {
			return total, err
		}
		rows, err := s.repo.ListCleanupCandidates(userID)
		if err != nil {
			return total, err
		}
		var doomed []string
		for _, row := range rows {
			if row.TransactionID != nil || row.ApprovedJSON != nil || row.Status == repo.MailStatusRejected {
				continue
			}
			header := &mail.MailHeader{FromAddr: row.FromAddr, Subject: row.Subject}
			body := ""
			if row.RawText != nil {
				body = *row.RawText
			}
			matched := false
			for _, rule := range rules {
				if rule.Enabled && mail.RuleMatches(rule, header, body) {
					matched = true
					break
				}
			}
			if !matched {
				doomed = append(doomed, row.ID)
			}
		}
		if len(doomed) == 0 {
			continue
		}
		n, err := s.repo.DeleteMessages(doomed)
		if err != nil {
			return total, err
		}
		total += int(n)
		slog.Info("mail cleanup", "user", userID, "deleted", n)
	}
	return total, nil
}

// --- AI re-parse job ---

func (s *Service) aiRunning(userID string) bool {
	s.aiMu.Lock()
	defer s.aiMu.Unlock()
	st := s.aiStates[userID]
	return st != nil && st.Running
}

// AIParseStatus returns a copy of the user's job state.
func (s *Service) AIParseStatus(userID string) model.AIParseStatus {
	s.aiMu.Lock()
	defer s.aiMu.Unlock()
	st := s.aiStates[userID]
	if st == nil {
		return model.AIParseStatus{}
	}
	return *st
}

// StartAIParse launches the background per-sender re-parse job.
func (s *Service) StartAIParse(userID string) (model.AIParseStatus, error) {
	s.aiMu.Lock()
	if st := s.aiStates[userID]; st != nil && st.Running {
		s.aiMu.Unlock()
		return model.AIParseStatus{}, errors.Conflict("AI parse is already running")
	}
	st := &model.AIParseStatus{Running: true, StartedAt: time.Now().Format(time.RFC3339)}
	s.aiStates[userID] = st
	s.aiMu.Unlock()
	go s.runAIParse(userID, st)
	return *st, nil
}

func (s *Service) runAIParse(userID string, st *model.AIParseStatus) {
	finish := func(msg string) {
		s.aiMu.Lock()
		st.Running = false
		st.CurrentSender = ""
		st.FinishedAt = time.Now().Format(time.RFC3339)
		if msg != "" {
			st.Error = msg
		}
		s.aiMu.Unlock()
	}
	defer finish("")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var rows []*repo.MailMessageRow
	for _, status := range []repo.MailStatus{repo.MailStatusPending, repo.MailStatusUnparsed} {
		list, err := s.repo.ListMessages(userID, &status, 500)
		if err != nil {
			finish(err.Error())
			return
		}
		rows = append(rows, list...)
	}
	groups := map[string][]*repo.MailMessageRow{}
	for _, row := range rows {
		groups[row.FromAddr] = append(groups[row.FromAddr], row)
	}
	rules, err := s.repo.ListRules(userID)
	if err != nil {
		finish(err.Error())
		return
	}
	settings := s.settings(userID)

	s.aiMu.Lock()
	st.TotalSenders = int64(len(groups))
	s.aiMu.Unlock()

	for sender, group := range groups {
		s.aiMu.Lock()
		st.CurrentSender = sender
		s.aiMu.Unlock()

		samples := s.samplesFor(ctx, userID, sender, group, 5)
		if len(samples) == 0 {
			s.aiMu.Lock()
			st.DoneSenders++
			s.aiMu.Unlock()
			continue
		}
		specs, genErr := s.generateTemplatesForce(ctx, userID, sender, samples, true)
		if genErr != nil {
			s.aiMu.Lock()
			st.Error = genErr.Error()
			st.DoneSenders++
			s.aiMu.Unlock()
			continue
		}
		applied := false
		for _, spec := range specs {
			tpl, compileErr := mail.CompileTemplate(spec)
			if compileErr != nil {
				continue
			}
			stored, storeErr := s.storeTemplate(userID, sender, spec, int64(len(samples)), settings.MinSamples)
			if storeErr != nil {
				continue
			}
			active := stored.Status == "active"
			// Use the new template on every pending row from this sender.
			for _, row := range group {
				if row.RawText == nil {
					continue
				}
				match, ok := tpl.Extract(row.Subject, *row.RawText)
				if !ok || !match.Complete || match.Extraction == nil || match.NeedsReview {
					continue
				}
				b, _ := json.Marshal(match.Extraction)
				parsed := string(b)
				status := repo.MailStatusPending
				if active && match.Extraction.AccountHint != nil {
					header := &mail.MailHeader{FromAddr: row.FromAddr, Subject: row.Subject}
					for _, rule := range rules {
						if rule.Enabled && rule.Action == repo.MailRuleActionAuto && mail.RuleMatches(rule, header, *row.RawText) {
							status = repo.MailStatusAuto
							break
						}
					}
				}
				if err := s.repo.UpdateMessageExtraction(userID, row.ID, &parsed, status, match.Extraction.Confidence, &stored.ID); err == nil {
					applied = true
				}
			}
			if applied {
				break
			}
		}
		s.aiMu.Lock()
		st.DoneSenders++
		s.aiMu.Unlock()
	}
	s.autoCreate(userID)
}

// samplesFor picks up to limit newest unrejected bodies for a sender.
func (s *Service) samplesFor(_ context.Context, userID, sender string, group []*repo.MailMessageRow, limit int) []mail.LLMSample {
	rows, err := s.repo.RecentMessagesFromSender(userID, sender, 20)
	if err != nil {
		rows = group
	}
	out := make([]mail.LLMSample, 0, limit)
	for _, row := range rows {
		if row.Status == repo.MailStatusRejected || row.RawText == nil {
			continue
		}
		out = append(out, mail.LLMSample{From: row.FromAddr, Subject: row.Subject, Body: *row.RawText})
		if len(out) >= limit {
			break
		}
	}
	return out
}

// storeTemplate persists a validated generated template (provisional until the
// configured sample count is reached).
func (s *Service) storeTemplate(userID, sender string, spec mail.TemplateSpec, samples, minSamples int64) (*repo.MailTemplateRow, error) {
	guards, err := json.Marshal(spec.Guards)
	if err != nil {
		return nil, err
	}
	patterns, err := json.Marshal(spec.Patterns)
	if err != nil {
		return nil, err
	}
	if minSamples < 1 {
		minSamples = 1
	}
	status := "provisional"
	if samples >= minSamples {
		status = "active"
	}
	return s.repo.CreateTemplate(userID, &repo.MailTemplateRow{
		Sender: sender, Name: spec.Name, GuardsJSON: string(guards),
		PatternsJSON: string(patterns), Status: status, SampleCount: samples,
	})
}

// ReapplyRules evaluates the current rules against mail already waiting in the
// queue, so a rule added after the mail arrived still takes effect. Returns the
// number of rows whose disposition changed.
func (s *Service) ReapplyRules(userID string) int {
	s.reapplyMu.Lock()
	defer s.reapplyMu.Unlock()

	rows, err := s.queuedMessages(userID)
	if err != nil {
		slog.Warn("reapply rules: could not list mail", "user", userID, "err", err)
		return 0
	}
	if len(rows) == 0 {
		return 0
	}
	rules, err := s.repo.ListRules(userID)
	if err != nil {
		slog.Warn("reapply rules: could not list rules", "user", userID, "err", err)
		return 0
	}
	changed := 0
	settings := s.settings(userID)
	for _, row := range rows {
		// ListMessages omits the raw body; a re-parse needs it.
		full := row
		if full.RawText == nil {
			if m, err := s.repo.GetMessage(userID, row.ID); err == nil && m != nil {
				full = m
			}
		}
		body := ""
		if full.RawText != nil {
			body = *full.RawText
		}
		header := &mail.MailHeader{FromAddr: full.FromAddr, Subject: full.Subject}

		// A new or edited template must reach mail that is already queued.
		extraction, status, templateID, _, _ := mail.Reparse(s.repo, userID, full, settings.MinSamples)
		if extraction != nil {
			if raw, err := json.Marshal(extraction); err == nil {
				parsed := string(raw)
				if err := s.repo.UpdateMessageExtraction(userID, row.ID, &parsed, status, extraction.Confidence, templateID); err != nil {
					slog.Warn("reapply rules: could not update extraction", "message", row.ID, "err", err)
				} else {
					row.ParsedJSON, row.Status = &parsed, status
					changed++
					slog.Info("mail re-parsed", "user", userID, "message_id", row.MessageID,
						"from", row.FromAddr, "status", string(status), "amount", extraction.Amount)
				}
			}
		}
		if rule := matchRule(rules, repo.MailRuleActionIgnore, header, body); rule != nil {
			if _, err := s.repo.ResolveMessage(userID, row.ID, repo.MailStatusIgnored, nil, nil); err == nil {
				changed++
				slog.Info("mail matched", "user", userID, "rule", rule.ID, "action", string(rule.Action),
					"message_id", row.MessageID, "from", row.FromAddr, "subject", row.Subject,
					"status", string(repo.MailStatusIgnored), "source", "existing")
			}
			continue
		}
		if rule := matchRule(rules, repo.MailRuleActionAuto, header, body); rule != nil {
			// Book only when the template extracted every required field cleanly;
			// autoCreate verifies the account and returns failures to review.
			if extraction != nil && status == repo.MailStatusPending {
				if _, err := s.repo.ResolveMessage(userID, row.ID, repo.MailStatusAuto, nil, nil); err == nil {
					changed++
					slog.Info("mail matched", "user", userID, "rule", rule.ID, "action", string(rule.Action),
						"message_id", row.MessageID, "from", row.FromAddr, "subject", row.Subject,
						"status", string(repo.MailStatusAuto), "source", "existing")
				}
			}
			continue
		}
		if rule := matchRule(rules, repo.MailRuleActionReview, header, body); rule != nil {
			slog.Debug("mail matched", "user", userID, "rule", rule.ID, "action", string(rule.Action),
				"message_id", row.MessageID, "from", row.FromAddr, "status", string(row.Status), "source", "existing")
		}
	}
	if changed > 0 {
		s.autoCreate(userID)
	}
	return changed
}

// queuedMessages returns the rows still waiting for a decision.
func (s *Service) queuedMessages(userID string) ([]*repo.MailMessageRow, error) {
	var out []*repo.MailMessageRow
	for _, status := range []repo.MailStatus{repo.MailStatusPending, repo.MailStatusUnparsed} {
		rows, err := s.repo.ListMessages(userID, &status, 5000)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	return out, nil
}

func matchRule(rules []*repo.MailRuleRow, action repo.MailRuleAction, header *mail.MailHeader, body string) *repo.MailRuleRow {
	for _, rule := range rules {
		if rule.Enabled && rule.Action == action && mail.RuleMatches(rule, header, body) {
			return rule
		}
	}
	return nil
}

// PollFullAsync scans the whole mailbox of every enabled account in the
// background; matched mail is stored in batches by the poll.
func (s *Service) PollFullAsync(userID string) string {
	go func() {
		accounts, err := s.repo.ListAccounts(userID)
		if err != nil {
			slog.Warn("full poll: could not list mailboxes", "user", userID, "err", err)
			return
		}
		for _, account := range accounts {
			if !account.Enabled {
				continue
			}
			if _, err := s.pollAccountMode(context.Background(), account, true, "full_poll"); err != nil {
				slog.Warn("full poll failed", "account", account.ID, "user", userID, "err", err)
				_ = s.repo.MarkPolled(account.ID, err.Error())
				continue
			}
			_ = s.repo.MarkPolled(account.ID, "")
		}
		s.ReapplyRules(userID)
		s.autoCreate(userID)
	}()
	return "started"
}

// generateTemplates is the LLM hook used by the poll. It enforces the user's
// per-sender cooldown before calling LiteLLM.
func (s *Service) generateTemplates(ctx context.Context, userID, sender string, samples []mail.LLMSample) ([]mail.TemplateSpec, error) {
	return s.generateTemplatesForce(ctx, userID, sender, samples, false)
}

// generateTemplatesForce runs the template generator; force bypasses the
// per-sender cooldown for an explicit user action.
func (s *Service) generateTemplatesForce(ctx context.Context, userID, sender string, samples []mail.LLMSample, force bool) ([]mail.TemplateSpec, error) {
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
	if !force {
		if last := s.lastLearn[key]; !last.IsZero() && time.Since(last) < cooldown {
			s.throttleMu.Unlock()
			return nil, nil
		}
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
		if _, err := s.pollAccount(context.Background(), account, "auto"); err != nil {
			slog.Warn("auto poll failed", "account", account.ID, "user", account.UserID, "err", err)
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
		out = append(out, s.ruleWire(userID, row))
	}
	return out, nil
}

// RulesPage returns one cursor page of mail rules and the next token.
func (s *Service) RulesPage(userID string, limit int64, pageToken string) ([]*model.MailRule, string, error) {
	values, err := cursorlib.Decode(pageToken)
	if err != nil {
		return nil, "", err
	}
	var cur *repo.MailRuleCursor
	if values != nil {
		if len(values) != 3 {
			return nil, "", errors.BadRequest("invalid pageToken")
		}
		position, err := strconv.ParseInt(values[0], 10, 64)
		if err != nil {
			return nil, "", errors.BadRequest("invalid pageToken")
		}
		cur = &repo.MailRuleCursor{Position: position, CreatedAt: values[1], ID: values[2]}
	}
	rows, next, err := s.repo.ListRulesPage(userID, limit, cur)
	if err != nil {
		return nil, "", err
	}
	out := make([]*model.MailRule, 0, len(rows))
	for _, row := range rows {
		out = append(out, s.ruleWire(userID, row))
	}
	return out, next, nil
}

func (s *Service) CreateRule(userID string, req model.ReqMailRule) (*model.MailRule, error) {
	input, err := ruleInput(req)
	if err != nil {
		return nil, err
	}
	spec, err := manualTemplateSpec(input.FromPattern, req.Template)
	if err != nil {
		return nil, err
	}
	// New rules join the end of the list unless a position was requested.
	if input.Position == 0 {
		rules, err := s.repo.ListRules(userID)
		if err != nil {
			return nil, err
		}
		var max int64
		for _, r := range rules {
			if r.Position > max {
				max = r.Position
			}
		}
		input.Position = max + 1
	}
	row, err := s.repo.CreateRule(userID, input)
	if err != nil {
		return nil, err
	}
	if err := s.saveManualTemplate(userID, input.FromPattern, spec); err != nil {
		return nil, err
	}
	go s.ReapplyRules(userID)
	return s.ruleWire(userID, row), nil
}

func (s *Service) UpdateRule(userID, id string, req model.ReqMailRule) error {
	input, err := ruleInput(req)
	if err != nil {
		return err
	}
	spec, err := manualTemplateSpec(input.FromPattern, req.Template)
	if err != nil {
		return err
	}
	oldFrom, err := s.ruleFromPattern(userID, id)
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
	// The template follows the sender pattern; a changed pattern leaves the old
	// template behind only if we do not remove it.
	if oldFrom != "" && oldFrom != input.FromPattern {
		if err := s.repo.DeleteManualTemplate(userID, oldFrom); err != nil {
			return err
		}
	}
	if err := s.saveManualTemplate(userID, input.FromPattern, spec); err != nil {
		return err
	}
	go s.ReapplyRules(userID)
	return nil
}

func (s *Service) DeleteRule(userID, id string) error {
	from, err := s.ruleFromPattern(userID, id)
	if err != nil {
		return err
	}
	ok, err := s.repo.DeleteRule(userID, id)
	if err != nil {
		return err
	}
	if !ok {
		return errors.NotFound("mail rule " + id + " not found")
	}
	if from != "" {
		if err := s.repo.DeleteManualTemplate(userID, from); err != nil {
			return err
		}
	}
	return nil
}

// ruleFromPattern finds a rule's sender condition before it is changed/deleted.
func (s *Service) ruleFromPattern(userID, id string) (string, error) {
	rules, err := s.repo.ListRules(userID)
	if err != nil {
		return "", err
	}
	for _, r := range rules {
		if r.ID == id {
			return r.FromPattern, nil
		}
	}
	return "", nil
}

// manualTemplateSpec validates the template carried by a rule request. A nil
// template means "no manual template", which removes any existing one.
func manualTemplateSpec(fromPattern string, p *model.MailTemplatePatterns) (*mail.TemplateSpec, error) {
	if p == nil {
		return nil, nil
	}
	if strings.TrimSpace(fromPattern) == "" {
		return nil, errors.BadRequest("a template needs a From condition")
	}
	spec := &mail.TemplateSpec{
		Name: "Manual · " + fromPattern,
		Patterns: mail.TemplatePatterns{
			Amount: p.Amount, Debit: p.Debit, Credit: p.Credit,
			Account: p.Account, Merchant: p.Merchant, Date: p.Date,
		},
	}
	if _, err := mail.CompileTemplate(*spec); err != nil {
		return nil, errors.BadRequest(err.Error())
	}
	return spec, nil
}

// saveManualTemplate stores or removes the manual template for a rule.
func (s *Service) saveManualTemplate(userID, fromPattern string, spec *mail.TemplateSpec) error {
	if spec == nil {
		if strings.TrimSpace(fromPattern) == "" {
			return nil
		}
		return s.repo.DeleteManualTemplate(userID, fromPattern)
	}
	guards, err := json.Marshal(spec.Guards)
	if err != nil {
		return err
	}
	patterns, err := json.Marshal(spec.Patterns)
	if err != nil {
		return err
	}
	_, err = s.repo.UpsertManualTemplate(userID, &repo.MailTemplateRow{
		Sender: fromPattern, Name: spec.Name,
		GuardsJSON: string(guards), PatternsJSON: string(patterns),
	})
	return err
}

// --- wire/validation helpers ---

func accountWire(row *repo.MailAccountRow) *model.MailAccount {
	return &model.MailAccount{
		ID: row.ID, Label: row.Label, IMAPHost: row.IMAPHost, IMAPPort: row.IMAPPort,
		Username: row.Username, Enabled: row.Enabled,
		LastPolledAt: row.LastPolledAt, LastError: row.LastError,
	}
}

// ruleWire attaches the rule's manual template, when it has one.
func (s *Service) ruleWire(userID string, row *repo.MailRuleRow) *model.MailRule {
	rule := ruleWire(row)
	if strings.TrimSpace(row.FromPattern) == "" {
		return rule
	}
	tpl, err := s.repo.GetManualTemplate(userID, row.FromPattern)
	if err != nil || tpl == nil {
		return rule
	}
	var spec mail.TemplateSpec
	if err := json.Unmarshal([]byte(tpl.PatternsJSON), &spec.Patterns); err != nil {
		return rule
	}
	rule.Template = &model.MailTemplatePatterns{
		Amount: []string(spec.Patterns.Amount), Debit: []string(spec.Patterns.Debit),
		Credit: []string(spec.Patterns.Credit), Account: []string(spec.Patterns.Account),
		Merchant: []string(spec.Patterns.Merchant), Date: []string(spec.Patterns.Date),
	}
	return rule
}

// ValidateTemplate previews a manual template against a pasted sample mail.
func (s *Service) ValidateTemplate(userID string, req model.ReqMailTemplateValidate) (*model.MailTemplateValidation, error) {
	if strings.TrimSpace(req.Subject) == "" && strings.TrimSpace(req.Body) == "" {
		return nil, errors.BadRequest("paste a sample mail to test the template")
	}
	spec := mail.TemplateSpec{Name: "manual", Patterns: mail.TemplatePatterns{
		Amount: req.Patterns.Amount, Debit: req.Patterns.Debit, Credit: req.Patterns.Credit,
		Account: req.Patterns.Account, Merchant: req.Patterns.Merchant, Date: req.Patterns.Date,
	}}
	v := mail.ValidateSpec(spec, req.Subject, req.Body)
	out := &model.MailTemplateValidation{Complete: v.Complete, Reason: v.Reason}
	for _, field := range v.Fields {
		check := model.MailTemplateFieldCheck{Field: field.Field}
		for _, p := range field.Patterns {
			check.Patterns = append(check.Patterns, model.MailTemplatePatternCheck{
				Pattern: p.Pattern, Matched: p.Matched, Value: p.Value, Error: p.Error,
			})
		}
		out.Fields = append(out.Fields, check)
	}
	if e := v.Extraction; e != nil {
		out.Amount = e.Amount
		out.Direction = string(e.Direction)
		out.Merchant = e.Merchant
		if e.AccountHint != nil {
			out.Account = *e.AccountHint
		}
		if e.OccurredAt != nil {
			out.Date = *e.OccurredAt
		}
	}
	return out, nil
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
	host, err := netguard.Hostname(raw)
	if err != nil {
		return "", err
	}
	return host, nil
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
