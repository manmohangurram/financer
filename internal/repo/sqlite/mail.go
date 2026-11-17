package sqlite

import (
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"github.com/mohan9182/financer/internal/repo"
	tim "github.com/mohan9182/financer/internal/time"
)

// MailRepo implements repo.MailRepo over SQLite.
type MailRepo struct {
	db *sql.DB
}

func NewMailRepo(db *sql.DB) *MailRepo {
	return &MailRepo{db: db}
}

func (r *MailRepo) CreateAccount(userID string, input *repo.NewMailAccount) (*repo.MailAccountRow, error) {
	id := uuid.New().String()
	now := tim.NowGoTS()
	if _, err := r.db.Exec(
		`INSERT INTO mail_accounts
			(id, user_id, label, imap_host, imap_port, username, secret_enc, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?)`,
		id, userID, input.Label, input.IMAPHost, input.IMAPPort, input.Username, input.SecretEnc, now,
	); err != nil {
		return nil, err
	}
	return &repo.MailAccountRow{
		ID: id, UserID: userID, Label: input.Label, IMAPHost: input.IMAPHost,
		IMAPPort: input.IMAPPort, Username: input.Username, SecretEnc: input.SecretEnc,
		Enabled: true, CreatedAt: now,
	}, nil
}

// UpdateAccount changes the editable mailbox fields. An empty secretEnc keeps
// the stored password.
func (r *MailRepo) UpdateAccount(userID, id, label, host string, port int64, username, secretEnc string, enabled bool) (bool, error) {
	res, err := r.db.Exec(
		`UPDATE mail_accounts SET
			label = ?, imap_host = ?, imap_port = ?, username = ?,
			secret_enc = COALESCE(NULLIF(?, ''), secret_enc), enabled = ?
		 WHERE id = ? AND user_id = ?`,
		label, host, port, username, secretEnc, boolToInt(enabled), id, userID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

const mailAccountColumns = `id, user_id, label, imap_host, imap_port, username, secret_enc,
	enabled, last_polled_at, last_error, created_at`

func (r *MailRepo) ListAccounts(userID string) ([]*repo.MailAccountRow, error) {
	rows, err := r.db.Query(
		"SELECT "+mailAccountColumns+" FROM mail_accounts WHERE user_id = ? ORDER BY created_at", userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMailAccounts(rows)
}

func (r *MailRepo) ListPollableAccounts() ([]*repo.MailAccountRow, error) {
	rows, err := r.db.Query(
		"SELECT " + mailAccountColumns + " FROM mail_accounts WHERE enabled = 1 ORDER BY created_at",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMailAccounts(rows)
}

func scanMailAccounts(rows *sql.Rows) ([]*repo.MailAccountRow, error) {
	var out []*repo.MailAccountRow
	for rows.Next() {
		var a repo.MailAccountRow
		var enabled int
		var lastPolled sql.NullString
		if err := rows.Scan(
			&a.ID, &a.UserID, &a.Label, &a.IMAPHost, &a.IMAPPort, &a.Username, &a.SecretEnc,
			&enabled, &lastPolled, &a.LastError, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		a.Enabled = enabled != 0
		if lastPolled.Valid {
			a.LastPolledAt = &lastPolled.String
		}
		out = append(out, &a)
	}
	return out, rows.Err()
}

func (r *MailRepo) DeleteAccount(userID, id string) (bool, error) {
	res, err := r.db.Exec("DELETE FROM mail_accounts WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *MailRepo) MarkPolled(accountID, errMsg string) error {
	_, err := r.db.Exec(
		"UPDATE mail_accounts SET last_polled_at = ?, last_error = ? WHERE id = ?",
		tim.NowGoTS(), errMsg, accountID,
	)
	return err
}

func (r *MailRepo) KnownMessageIDs(userID string, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+1)
	args = append(args, userID)
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := r.db.Query(
		"SELECT message_id FROM mail_messages WHERE user_id = ? AND message_id IN ("+placeholders+")", args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *MailRepo) InsertMessage(userID string, msg *repo.NewMailMessage) (bool, error) {
	res, err := r.db.Exec(
		`INSERT OR IGNORE INTO mail_messages
			(id, user_id, mail_account_id, message_id, from_addr, received_at, subject,
			 raw_text, status, parsed_json, confidence, extractor_source, template_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), userID, msg.MailAccountID, msg.MessageID, msg.FromAddr,
		msg.ReceivedAt, msg.Subject, msg.RawText, string(msg.Status), msg.ParsedJSON,
		msg.Confidence, msg.ExtractorSource, msg.TemplateID, tim.NowGoTS(),
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

const mailMessageColumns = `id, message_id, mail_account_id, from_addr, subject, received_at, status, parsed_json,
	approved_json, COALESCE(confidence, 0), raw_text, transaction_id, template_id`

func (r *MailRepo) ListMessages(userID string, status *repo.MailStatus, limit int64) ([]*repo.MailMessageRow, error) {
	var statusVal any
	if status != nil {
		statusVal = string(*status)
	}
	rows, err := r.db.Query(
		"SELECT "+mailMessageColumns+` FROM mail_messages
		 WHERE user_id = ? AND (? IS NULL OR status = ?)
		 ORDER BY created_at DESC LIMIT ?`,
		userID, statusVal, statusVal, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*repo.MailMessageRow
	for rows.Next() {
		m, err := scanMailMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *MailRepo) GetMessage(userID, id string) (*repo.MailMessageRow, error) {
	rows, err := r.db.Query(
		"SELECT "+mailMessageColumns+" FROM mail_messages WHERE id = ? AND user_id = ?", id, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	m, err := scanMailMessage(rows)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func scanMailMessage(rows *sql.Rows) (*repo.MailMessageRow, error) {
	var m repo.MailMessageRow
	var status string
	var receivedAt, parsed, approved, rawText, txnID, templateID sql.NullString
	if err := rows.Scan(
		&m.ID, &m.MessageID, &m.MailAccountID, &m.FromAddr, &m.Subject, &receivedAt, &status,
		&parsed, &approved, &m.Confidence, &rawText, &txnID, &templateID,
	); err != nil {
		return nil, err
	}
	m.Status = repo.MailStatus(status)
	if receivedAt.Valid {
		m.ReceivedAt = &receivedAt.String
	}
	if parsed.Valid {
		m.ParsedJSON = &parsed.String
	}
	if approved.Valid {
		m.ApprovedJSON = &approved.String
	}
	if rawText.Valid {
		m.RawText = &rawText.String
	}
	if txnID.Valid {
		m.TransactionID = &txnID.String
	}
	if templateID.Valid {
		m.TemplateID = &templateID.String
	}
	return &m, nil
}

func (r *MailRepo) ResolveMessage(userID, id string, status repo.MailStatus, approvedJSON, transactionID *string) (bool, error) {
	res, err := r.db.Exec(
		`UPDATE mail_messages
		 SET status = ?, approved_json = COALESCE(?, approved_json),
		     transaction_id = COALESCE(?, transaction_id)
		 WHERE id = ? AND user_id = ?`,
		string(status), approvedJSON, transactionID, id, userID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

const mailRuleColumns = "id, position, enabled, from_pattern, subject_contains, body_contains, action, account_id, category_ids"

func (r *MailRepo) ListRules(userID string) ([]*repo.MailRuleRow, error) {
	rows, err := r.db.Query(
		"SELECT "+mailRuleColumns+" FROM mail_rules WHERE user_id = ? ORDER BY position, created_at", userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*repo.MailRuleRow
	for rows.Next() {
		rule, err := scanMailRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

func (r *MailRepo) CreateRule(userID string, input *repo.MailRuleInput) (*repo.MailRuleRow, error) {
	id := uuid.New().String()
	var accountID any
	if input.AccountID != nil {
		accountID = *input.AccountID
	}
	if _, err := r.db.Exec(
		`INSERT INTO mail_rules
			(id, user_id, position, enabled, from_pattern, subject_contains, body_contains, action, account_id, category_ids, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, userID, input.Position, boolToInt(input.Enabled), input.FromPattern,
		input.SubjectContains, input.BodyContains, string(input.Action), accountID,
		marshalStrings(input.CategoryIDs), tim.NowGoTS(),
	); err != nil {
		return nil, err
	}
	return &repo.MailRuleRow{
		ID: id, Position: input.Position, Enabled: input.Enabled,
		FromPattern: input.FromPattern, SubjectContains: input.SubjectContains,
		BodyContains: input.BodyContains, Action: input.Action,
		AccountID: input.AccountID, CategoryIDs: input.CategoryIDs,
	}, nil
}

func (r *MailRepo) UpdateRule(userID, id string, input *repo.MailRuleInput) (bool, error) {
	var accountID any
	if input.AccountID != nil {
		accountID = *input.AccountID
	}
	res, err := r.db.Exec(
		`UPDATE mail_rules SET position = ?, enabled = ?, from_pattern = ?, subject_contains = ?,
			body_contains = ?, action = ?, account_id = ?, category_ids = ?
		 WHERE id = ? AND user_id = ?`,
		input.Position, boolToInt(input.Enabled), input.FromPattern, input.SubjectContains,
		input.BodyContains, string(input.Action), accountID, marshalStrings(input.CategoryIDs),
		id, userID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *MailRepo) DeleteRule(userID, id string) (bool, error) {
	res, err := r.db.Exec("DELETE FROM mail_rules WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *MailRepo) SweepBodies() (int64, error) {
	res, err := r.db.Exec(
		"UPDATE mail_messages SET raw_text = NULL WHERE raw_text IS NOT NULL AND status NOT IN ('PENDING', 'UNPARSED')",
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func scanMailRule(rows *sql.Rows) (*repo.MailRuleRow, error) {
	var rule repo.MailRuleRow
	var enabled int
	var action string
	var accountID sql.NullString
	var catJSON string
	if err := rows.Scan(
		&rule.ID, &rule.Position, &enabled, &rule.FromPattern, &rule.SubjectContains,
		&rule.BodyContains, &action, &accountID, &catJSON,
	); err != nil {
		return nil, err
	}
	rule.Enabled = enabled != 0
	rule.Action = repo.MailRuleAction(action)
	if rule.Action != repo.MailRuleActionIgnore && rule.Action != repo.MailRuleActionAuto {
		rule.Action = repo.MailRuleActionReview
	}
	if accountID.Valid {
		rule.AccountID = &accountID.String
	}
	rule.CategoryIDs = unmarshalStrings(catJSON)
	return &rule, nil
}

func (r *MailRepo) ListTemplates(userID, sender string) ([]*repo.MailTemplateRow, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, sender, name, guards_json, patterns_json, status,
		        sample_count, successes, failures, created_at, last_used_at, last_attempt_at
		 FROM mail_templates
		 WHERE user_id = ? AND sender = ? AND status != 'disabled'
		 ORDER BY sample_count DESC, created_at DESC`,
		userID, sender,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*repo.MailTemplateRow
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *MailRepo) CreateTemplate(userID string, row *repo.MailTemplateRow) (*repo.MailTemplateRow, error) {
	id := uuid.New().String()
	now := tim.NowGoTS()
	if _, err := r.db.Exec(
		`INSERT INTO mail_templates
			(id, user_id, sender, name, guards_json, patterns_json, status, sample_count, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, userID, row.Sender, row.Name, row.GuardsJSON, row.PatternsJSON,
		row.Status, row.SampleCount, now,
	); err != nil {
		return nil, err
	}
	row.ID = id
	row.UserID = userID
	row.CreatedAt = now
	return row, nil
}

func (r *MailRepo) RecordTemplateResult(id string, success bool) error {
	if success {
		_, err := r.db.Exec(
			`UPDATE mail_templates SET successes = successes + 1, last_used_at = ? WHERE id = ?`,
			tim.NowGoTS(), id,
		)
		return err
	}
	_, err := r.db.Exec(`UPDATE mail_templates SET failures = failures + 1 WHERE id = ?`, id)
	return err
}

func (r *MailRepo) AddTemplateSample(id string) (int64, error) {
	if _, err := r.db.Exec(`UPDATE mail_templates SET sample_count = sample_count + 1 WHERE id = ?`, id); err != nil {
		return 0, err
	}
	var count int64
	if err := r.db.QueryRow(`SELECT sample_count FROM mail_templates WHERE id = ?`, id).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *MailRepo) PromoteTemplate(id string, sampleCount int64) error {
	_, err := r.db.Exec(
		`UPDATE mail_templates SET status = 'active', sample_count = ? WHERE id = ?`,
		sampleCount, id,
	)
	return err
}

func (r *MailRepo) RecentMessagesFromSender(userID, sender string, limit int64) ([]*repo.MailMessageRow, error) {
	rows, err := r.db.Query(
		"SELECT "+mailMessageColumns+` FROM mail_messages
		 WHERE user_id = ? AND from_addr = ? AND status != 'IGNORED'
		 ORDER BY created_at DESC LIMIT ?`,
		userID, sender, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*repo.MailMessageRow
	for rows.Next() {
		m, err := scanMailMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *MailRepo) SetMessageTemplate(id, templateID string) error {
	_, err := r.db.Exec("UPDATE mail_messages SET template_id = ? WHERE id = ?", templateID, id)
	return err
}

func scanTemplate(rows *sql.Rows) (*repo.MailTemplateRow, error) {
	var t repo.MailTemplateRow
	var lastUsed, lastAttempt sql.NullString
	if err := rows.Scan(
		&t.ID, &t.UserID, &t.Sender, &t.Name, &t.GuardsJSON, &t.PatternsJSON,
		&t.Status, &t.SampleCount, &t.Successes, &t.Failures, &t.CreatedAt,
		&lastUsed, &lastAttempt,
	); err != nil {
		return nil, err
	}
	if lastUsed.Valid {
		t.LastUsedAt = &lastUsed.String
	}
	if lastAttempt.Valid {
		t.LastAttemptAt = &lastAttempt.String
	}
	return &t, nil
}

func marshalStrings(v []string) string {
	if v == nil {
		return "[]"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func unmarshalStrings(raw string) []string {
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
