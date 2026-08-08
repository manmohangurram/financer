package repository

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mohan9182/financer/api"
)

type TxnCursor struct {
	OccurredAt time.Time
	ID         string
}

func encodeTxnCursor(c TxnCursor) string {
	raw, _ := json.Marshal(c)
	return base64.URLEncoding.EncodeToString(raw)
}

func decodeTxnCursor(token string) (*TxnCursor, error) {
	raw, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}
	var c TxnCursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

type TransactionRepository struct {
	*BaseRepository
}

func NewTransactionRepository(base *BaseRepository) *TransactionRepository {
	return &TransactionRepository{BaseRepository: base}
}

type CreateTransactionInput struct {
	Txn         *api.TransactionResponse
	CategoryIDs []string
}

type UpdateTransactionInput struct {
	Txn         *api.TransactionResponse
	CategoryIDs []string
}

func scanTransaction(row scannable) (*api.TransactionResponse, error) {
	var txn api.TransactionResponse
	var occurredAt, createdAt time.Time
	var txnType int
	err := row.Scan(&txn.Id, &txn.Name, &txn.Amount, &txnType, &occurredAt, &txn.AccountId, &createdAt)
	if err != nil {
		return nil, err
	}
	txn.Type = api.TransactionType(txnType)
	txn.OccurredAt = occurredAt
	txn.CreatedAt = createdAt
	return &txn, nil
}

func (r *TransactionRepository) GetByID(ctx context.Context, id string) (*api.TransactionResponse, error) {
	txn, err := QueryOne(ctx, r.readDB,
		`SELECT id, name, amount, type, occurred_at, account_id, created_at
		 FROM transactions WHERE id = ?`,
		scanTransaction, id,
	)
	if err != nil {
		return nil, fmt.Errorf("querying transaction: %w", err)
	}
	if txn == nil {
		return nil, nil
	}
	r.populateLink(ctx, txn)
	r.populateCategories(ctx, txn)
	return txn, nil
}

type ListTxnResult struct {
	Transactions  []*api.TransactionResponse
	NextPageToken string
	TotalCount    int32
}

type TxnListFilter struct {
	AccountID    string
	CategoryIDs  []string
	TxnType      api.TransactionType
	DateFrom     time.Time
	DateTo       time.Time
	MinAmount    float32
	MaxAmount    float32
	Name         string
	NameMatch    string
	PageSize     int32
	PageToken    string
	SortBy       string // "debit" | "credit" (primary type first)
	SortDir      string // "asc" | "desc" (default desc)
	Offset       int32  // used with SortBy (offset pagination)
}

// buildTxnWhere returns the WHERE clause and args (userID is the first ?).
func buildTxnWhere(f TxnListFilter) (string, []any) {
	query := "WHERE t.user_id = ?"
	var args []any
	if f.AccountID != "" {
		query += " AND t.account_id = ?"
		args = append(args, f.AccountID)
	}
	if len(f.CategoryIDs) > 0 {
		placeholders := make([]string, len(f.CategoryIDs))
		for i, id := range f.CategoryIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		query += " AND EXISTS (SELECT 1 FROM transaction_categories tc WHERE tc.transaction_id = t.id AND tc.category_id IN (" + strings.Join(placeholders, ",") + "))"
	}
	if f.TxnType != api.TransactionType(0) {
		query += " AND t.type = ?"
		args = append(args, int(f.TxnType))
	}
	if !f.DateFrom.IsZero() {
		query += " AND t.occurred_at >= ?"
		args = append(args, f.DateFrom)
	}
	if !f.DateTo.IsZero() {
		query += " AND t.occurred_at < ?"
		args = append(args, f.DateTo.Add(24*time.Hour))
	}
	if f.MinAmount > 0 {
		query += " AND t.amount >= ?"
		args = append(args, f.MinAmount)
	}
	if f.MaxAmount > 0 {
		query += " AND t.amount <= ?"
		args = append(args, f.MaxAmount)
	}
	if f.Name != "" {
		if f.NameMatch == "exact" {
			query += " AND LOWER(t.name) = LOWER(?)"
			args = append(args, f.Name)
		} else {
			query += " AND LOWER(t.name) LIKE '%' || LOWER(?) || '%'"
			args = append(args, likeEscape(f.Name))
		}
	}
	return query, args
}

type listTxnRow struct {
	txn         api.TransactionResponse
	linkID      string
	categoryIDs []string
	totalCount  int32
}

func scanListRow(row scannable) (*listTxnRow, error) {
	var r listTxnRow
	var occurredAt, createdAt time.Time
	var txnType int
	var catGroup sql.NullString
	if err := row.Scan(&r.txn.Id, &r.txn.Name, &r.txn.Amount, &txnType, &occurredAt, &r.txn.AccountId, &createdAt,
		&r.linkID, &catGroup, &r.totalCount); err != nil {
		return nil, err
	}
	r.txn.Type = api.TransactionType(txnType)
	r.txn.OccurredAt = occurredAt
	r.txn.CreatedAt = createdAt
	if catGroup.Valid && catGroup.String != "" {
		r.categoryIDs = strings.Split(catGroup.String, ",")
	}
	return &r, nil
}

func (r *TransactionRepository) List(ctx context.Context, userID string, f TxnListFilter) (*ListTxnResult, error) {
	where, args := buildTxnWhere(f)
	args = append([]any{userID}, args...)

	query := `SELECT t.id, t.name, t.amount, t.type, t.occurred_at, t.account_id, t.created_at,
	              COALESCE(l.id, ''),
	              (SELECT GROUP_CONCAT(tc.category_id) FROM transaction_categories tc WHERE tc.transaction_id = t.id),
	              COUNT(*) OVER () AS total
	          FROM transactions t
	          LEFT JOIN transfer_links l ON l.debit_transaction_id = t.id OR l.credit_transaction_id = t.id ` + where

	if f.SortBy != "" {
		// Amount sort: primary type first in the chosen direction; the other
		// type in the opposite direction. Offset pagination.
		primary := 0
		if f.SortBy == "credit" {
			primary = 1
		}
		dir := "DESC"
		if f.SortDir == "asc" {
			dir = "ASC"
		}
		// -amount reverses the secondary group relative to the primary.
		query += fmt.Sprintf(` ORDER BY CASE WHEN t.type = %d THEN 0 ELSE 1 END,
			CASE WHEN t.type = %d THEN t.amount ELSE -t.amount END %s, t.id ASC`, primary, primary, dir)
		if f.PageSize > 0 {
			query += fmt.Sprintf(" LIMIT %d OFFSET %d", f.PageSize, f.Offset)
		}
	} else {
		if f.PageToken != "" {
			cursor, err := decodeTxnCursor(f.PageToken)
			if err == nil {
				query += " AND (t.occurred_at < ? OR (t.occurred_at = ? AND t.id < ?))"
				args = append(args, cursor.OccurredAt, cursor.OccurredAt, cursor.ID)
			}
		}

		query += " ORDER BY t.occurred_at DESC, t.id DESC"

		if f.PageSize > 0 {
			query += fmt.Sprintf(" LIMIT %d", f.PageSize+1)
		}
	}

	rows, err := QueryAll(ctx, r.readDB, query, scanListRow, args...)
	if err != nil {
		return nil, fmt.Errorf("listing transactions: %w", err)
	}

	txns := make([]*api.TransactionResponse, 0, len(rows))
	var total int32
	for _, row := range rows {
		txn := &row.txn
		txn.LinkedTransferId = row.linkID
		txn.CategoryIds = row.categoryIDs
		total = row.totalCount
		txns = append(txns, txn)
	}

	var nextToken string
	if f.SortBy == "" && f.PageSize > 0 && int32(len(txns)) > f.PageSize {
		txns = txns[:f.PageSize]
		last := txns[len(txns)-1]
		nextToken = encodeTxnCursor(TxnCursor{
			OccurredAt: last.OccurredAt,
			ID:         last.Id,
		})
	}

	return &ListTxnResult{Transactions: txns, NextPageToken: nextToken, TotalCount: total}, nil
}

// populateLink attaches the transfer link id (if any) to a single transaction.
func (r *TransactionRepository) populateLink(ctx context.Context, txn *api.TransactionResponse) {
	var linkID sql.NullString
	r.readDB.QueryRowContext(ctx,
		`SELECT id FROM transfer_links
		 WHERE debit_transaction_id = ? OR credit_transaction_id = ?`, txn.Id, txn.Id,
	).Scan(&linkID)
	if linkID.Valid {
		txn.LinkedTransferId = linkID.String
	}
}

func (r *TransactionRepository) populateCategories(ctx context.Context, txns ...*api.TransactionResponse) {
	if len(txns) == 0 {
		return
	}
	placeholders := make([]string, len(txns))
	args := make([]any, len(txns))
	for i, t := range txns {
		placeholders[i] = "?"
		args[i] = t.Id
	}
	rows, err := r.readDB.QueryContext(ctx,
		`SELECT transaction_id, category_id FROM transaction_categories
		 WHERE transaction_id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return
	}
	defer rows.Close()
	byTxn := make(map[string][]string)
	for rows.Next() {
		var txnID, catID string
		if err := rows.Scan(&txnID, &catID); err != nil {
			return
		}
		byTxn[txnID] = append(byTxn[txnID], catID)
	}
	for _, t := range txns {
		t.CategoryIds = byTxn[t.Id]
	}
}

func (r *TransactionRepository) GetByIDForTransfer(ctx context.Context, id string) (txnType int, amount float64, accountID string, err error) {
	err = r.readDB.QueryRowContext(ctx,
		`SELECT type, amount, account_id FROM transactions WHERE id = ?`, id,
	).Scan(&txnType, &amount, &accountID)
	return txnType, amount, accountID, err
}

// FindTransferCounterpart returns the unlinked transaction in accountID with
// the given type/amount closest in date to around (within ±3 days).
func (r *TransactionRepository) FindTransferCounterpart(ctx context.Context, userID, accountID string, txnType api.TransactionType, amount float32, around time.Time) (*api.TransactionResponse, error) {
	aroundStr := around.UTC().Format("2006-01-02 15:04:05")
	query := `SELECT t.id, t.name, t.amount, t.type, t.occurred_at, t.account_id, t.created_at
		FROM transactions t
		WHERE t.user_id = ? AND t.account_id = ? AND t.type = ? AND t.amount = ?
		  AND t.occurred_at >= datetime(?, '-3 days') AND t.occurred_at <= datetime(?, '+3 days')
		  AND NOT EXISTS (SELECT 1 FROM transfer_links l WHERE l.debit_transaction_id = t.id OR l.credit_transaction_id = t.id)
		ORDER BY ABS(CAST(strftime('%s', t.occurred_at) AS INTEGER) - CAST(strftime('%s', ?) AS INTEGER))
		LIMIT 1`
	return QueryOne(ctx, r.readDB, query, scanTransaction, userID, accountID, int(txnType), amount, aroundStr, aroundStr, aroundStr)
}

// IsTransferLinked reports whether txnID is already part of a transfer link.
func (r *TransactionRepository) IsTransferLinked(ctx context.Context, userID, txnID string) (bool, error) {
	var one int
	err := r.readDB.QueryRowContext(ctx,
		`SELECT 1 FROM transfer_links l WHERE l.debit_transaction_id = ? OR l.credit_transaction_id = ? LIMIT 1`,
		txnID, txnID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Create inserts transactions with INSERT OR IGNORE and reports which ids were
// actually inserted. A row whose (user_id, external_id) already exists is
// skipped silently (idempotent import) and is not in the returned set.
func (r *TransactionRepository) Create(ctx context.Context, userID string, inputs []CreateTransactionInput) (map[string]bool, []error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	var errors []error
	inserted := make(map[string]bool)

	r.ExecInTx(ctx, func(tx *Tx) error {
		stmt, err := tx.PrepareContext(ctx,
			`INSERT OR IGNORE INTO transactions (id, user_id, name, amount, type, occurred_at, account_id, created_at, external_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		)
		if err != nil {
			errors = append(errors, fmt.Errorf("preparing statement: %w", err))
			return err
		}
		defer stmt.Close()

		catStmt, err := tx.PrepareContext(ctx,
			`INSERT OR IGNORE INTO transaction_categories (transaction_id, category_id) VALUES (?, ?)`,
		)
		if err != nil {
			errors = append(errors, fmt.Errorf("preparing category statement: %w", err))
			return err
		}
		defer catStmt.Close()

		for _, input := range inputs {
			txn := input.Txn
			var ext sql.NullString
			if txn.ExternalId != "" {
				ext = sql.NullString{String: txn.ExternalId, Valid: true}
			}
			res, err := stmt.ExecContext(ctx, txn.Id, userID, txn.Name, txn.Amount, int(txn.Type), txn.OccurredAt, txn.AccountId, txn.CreatedAt, ext)
			if err != nil {
				errors = append(errors, fmt.Errorf("failed to create %s: %w", txn.Id, err))
				continue
			}
			if n, _ := res.RowsAffected(); n == 0 {
				continue // duplicate external_id — skip silently
			}
			inserted[txn.Id] = true
			for _, catID := range input.CategoryIDs {
				if _, err := catStmt.ExecContext(ctx, txn.Id, catID); err != nil {
					errors = append(errors, fmt.Errorf("failed to link category %s to %s: %w", catID, txn.Id, err))
				}
			}
		}
		if len(errors) > 0 {
			return fmt.Errorf("some transactions failed")
		}
		return nil
	})

	return inserted, errors
}

func (r *TransactionRepository) Update(ctx context.Context, inputs []UpdateTransactionInput) []error {
	if len(inputs) == 0 {
		return nil
	}

	var errors []error

	r.ExecInTx(ctx, func(tx *Tx) error {
		stmt, err := tx.PrepareContext(ctx,
			`UPDATE transactions SET
				name = COALESCE(NULLIF(?, ''), name),
				amount = CASE WHEN ? != 0 THEN ? ELSE amount END,
				type = CASE WHEN ? != 0 THEN ? ELSE type END,
				occurred_at = COALESCE(?, occurred_at),
				account_id = COALESCE(NULLIF(?, ''), account_id)
			 WHERE id = ?`,
		)
		if err != nil {
			errors = append(errors, fmt.Errorf("preparing statement: %w", err))
			return err
		}
		defer stmt.Close()

		for _, input := range inputs {
			txn := input.Txn
			res, err := stmt.ExecContext(ctx,
				txn.Name, txn.Amount, txn.Amount,
				int(txn.Type), int(txn.Type),
				nullableTime(txn.OccurredAt),
				txn.AccountId, txn.Id,
			)
			if err != nil {
				errors = append(errors, fmt.Errorf("failed to update %s: %w", txn.Id, err))
				continue
			}
			rows, _ := res.RowsAffected()
			if rows == 0 {
				errors = append(errors, fmt.Errorf("transaction %s not found", txn.Id))
				continue
			}
			// Replace categories: an update carries the full category set
			// (the client sends current categoryIds on every edit).
			if _, err := tx.ExecContext(ctx, `DELETE FROM transaction_categories WHERE transaction_id = ?`, txn.Id); err != nil {
				errors = append(errors, fmt.Errorf("failed to clear categories for %s: %w", txn.Id, err))
				continue
			}
			for _, catID := range input.CategoryIDs {
				if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO transaction_categories (transaction_id, category_id) VALUES (?, ?)`, txn.Id, catID); err != nil {
					errors = append(errors, fmt.Errorf("failed to link category %s to %s: %w", catID, txn.Id, err))
				}
			}
		}
		if len(errors) > 0 {
			return fmt.Errorf("some updates failed")
		}
		return nil
	})

	return errors
}

func (r *TransactionRepository) Delete(ctx context.Context, ids []string) []error {
	if len(ids) == 0 {
		return nil
	}

	var errors []error

	r.ExecInTx(ctx, func(tx *Tx) error {
		stmt, err := tx.PrepareContext(ctx, `DELETE FROM transactions WHERE id = ?`)
		if err != nil {
			errors = append(errors, fmt.Errorf("preparing statement: %w", err))
			return err
		}
		defer stmt.Close()

		for _, id := range ids {
			if _, err := stmt.ExecContext(ctx, id); err != nil {
				errors = append(errors, fmt.Errorf("failed to delete %s: %w", id, err))
			}
		}
		if len(errors) > 0 {
			return fmt.Errorf("some deletions failed")
		}
		return nil
	})

	return errors
}

func (r *TransactionRepository) GetByIDBatch(ctx context.Context, txnIDs []string) ([]*api.TransactionResponse, error) {
	if len(txnIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(txnIDs))
	args := make([]any, len(txnIDs))
	for i, id := range txnIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf(
		`SELECT id, name, amount, type, occurred_at, account_id, created_at FROM transactions WHERE id IN (%s)`,
		strings.Join(placeholders, ","),
	)
	return QueryAll(ctx, r.readDB, query, scanTransaction, args...)
}

func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

type SpendingFilter struct {
	Granularity string // "day" | "month"
	From        time.Time
	To          time.Time
	AccountID   string
}

type SpendingBucketRow struct {
	Key    string
	Amount float32
}

// spendingWhere is the LEFT JOIN chain used by both bucket and category queries:
// it exposes the transfer counterpart account (a_other) so a transfer is
// excluded unless one side is a debt account (account_type 3=CREDIT_CARD, 4=LOAN).
func spendingWhere() string {
	return `LEFT JOIN transfer_links tl ON tl.debit_transaction_id = t.id OR tl.credit_transaction_id = t.id
		LEFT JOIN accounts a_self ON a_self.id = t.account_id
		LEFT JOIN accounts a_other ON a_other.id = CASE WHEN tl.debit_transaction_id = t.id THEN tl.credit_transaction_id ELSE tl.debit_transaction_id END`
}

func spendingRangeClause(f SpendingFilter) string {
	var s strings.Builder
	if !f.From.IsZero() {
		s.WriteString(" AND t.occurred_at >= ?")
	}
	if !f.To.IsZero() {
		s.WriteString(" AND t.occurred_at < ?")
	}
	if f.AccountID != "" {
		s.WriteString(" AND t.account_id = ?")
	}
	return s.String()
}

// SpendingBuckets sums debit spending per day/month, excluding non-debt
// transfers. occurred_at is stored as "YYYY-MM-DD HH:MM:SS +0000 UTC" (driver
// format); substr extracts the day/month prefix without relying on date().
func (r *TransactionRepository) SpendingBuckets(ctx context.Context, userID string, f SpendingFilter) ([]*SpendingBucketRow, error) {
	keyExpr := "substr(t.occurred_at, 1, 10)"
	if f.Granularity == "month" {
		keyExpr = "substr(t.occurred_at, 1, 7)"
	}
	query := fmt.Sprintf(`SELECT %s AS key, COALESCE(SUM(t.amount), 0) AS amount
		FROM transactions t
		%s
		WHERE t.user_id = ? AND t.type = 0
		  AND (tl.id IS NULL OR a_self.account_type IN (3,4) OR a_other.account_type IN (3,4))
		%s
		GROUP BY key ORDER BY key`, keyExpr, spendingWhere(), spendingRangeClause(f))
	args := []any{userID}
	if !f.From.IsZero() {
		args = append(args, f.From)
	}
	if !f.To.IsZero() {
		args = append(args, f.To)
	}
	if f.AccountID != "" {
		args = append(args, f.AccountID)
	}
	return QueryAll(ctx, r.readDB, query, func(row scannable) (*SpendingBucketRow, error) {
		var b SpendingBucketRow
		return &b, row.Scan(&b.Key, &b.Amount)
	}, args...)
}

// SpendingCategories sums debit/credit per category, with the same transfer
// exclusion (non-debt transfers excluded; debt-account transfers included).
func (r *TransactionRepository) SpendingCategories(ctx context.Context, userID string, f SpendingFilter) ([]*api.SpendingCategory, error) {
	query := fmt.Sprintf(`SELECT COALESCE(c.id, '__uncategorized__'), COALESCE(c.name, 'Uncategorized'),
		COALESCE(SUM(CASE WHEN t.type = 0 THEN t.amount ELSE 0 END), 0) AS debit,
		COALESCE(SUM(CASE WHEN t.type = 1 THEN t.amount ELSE 0 END), 0) AS credit
		FROM transactions t
		LEFT JOIN transaction_categories tc ON tc.transaction_id = t.id
		LEFT JOIN categories c ON c.id = tc.category_id
		%s
		WHERE t.user_id = ? AND (tl.id IS NULL OR a_self.account_type IN (3,4) OR a_other.account_type IN (3,4))
		%s
		GROUP BY COALESCE(c.id, '__uncategorized__') ORDER BY debit DESC`, spendingWhere(), spendingRangeClause(f))
	args := []any{userID}
	if !f.From.IsZero() {
		args = append(args, f.From)
	}
	if !f.To.IsZero() {
		args = append(args, f.To)
	}
	if f.AccountID != "" {
		args = append(args, f.AccountID)
	}
	return QueryAll(ctx, r.readDB, query, func(row scannable) (*api.SpendingCategory, error) {
		var c api.SpendingCategory
		return &c, row.Scan(&c.Id, &c.Name, &c.Debit, &c.Credit)
	}, args...)
}

// SearchByRule runs a rule's conditions as a SQL query against the user's
// transactions, returning matching rows (newest first) up to limit.
func (r *TransactionRepository) SearchByRule(ctx context.Context, userID string, logic string, conds []RuleCondData, limit int32) ([]*api.TransactionResponse, error) {
	query, args, err := buildRuleQuery(userID, logic, conds, limit)
	if err != nil {
		return nil, err
	}
	results, err := QueryAll(ctx, r.readDB, query, scanTransaction, args...)
	if err != nil {
		return nil, fmt.Errorf("searching by rule: %w", err)
	}
	return results, nil
}

// buildRuleQuery filters the user's transactions by a rule's conditions
// directly in SQL instead of scanning rows in Go.
func buildRuleQuery(userID string, logic string, conds []RuleCondData, limit int32) (string, []any, error) {
	query := `SELECT t.id, t.name, t.amount, t.type, t.occurred_at, t.account_id, t.created_at FROM transactions t WHERE t.user_id = ?`
	args := []any{userID}

	clauses := make([]string, 0, len(conds))
	for _, c := range conds {
		clause, cargs, err := buildConditionClause(c)
		if err != nil {
			return "", nil, err
		}
		clauses = append(clauses, "("+clause+")")
		args = append(args, cargs...)
	}
	if len(clauses) > 0 {
		sep := " OR "
		if strings.EqualFold(logic, "AND") {
			sep = " AND "
		}
		query += " AND (" + strings.Join(clauses, sep) + ")"
	}

	query += " ORDER BY t.occurred_at DESC, t.id DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	return query, args, nil
}

func buildConditionClause(c RuleCondData) (string, []any, error) {
	if c.MatchField == "RULE_MATCH_FIELD_CATEGORY" {
		sub, args, err := textClause("c.name", c.Operator, c.Pattern)
		if err != nil {
			return "", nil, err
		}
		return "EXISTS (SELECT 1 FROM transaction_categories tc JOIN categories c ON c.id = tc.category_id WHERE tc.transaction_id = t.id AND " + sub + ")", args, nil
	}
	if c.MatchField == "RULE_MATCH_FIELD_AMOUNT" || c.MatchField == "RULE_MATCH_FIELD_TYPE" {
		return numericClause(columnFor(c.MatchField), c.Operator, c.Pattern)
	}
	return textClause(columnFor(c.MatchField), c.Operator, c.Pattern)
}

func columnFor(field string) string {
	switch field {
	case "RULE_MATCH_FIELD_ACCOUNT":
		return "t.account_id"
	case "RULE_MATCH_FIELD_AMOUNT":
		return "t.amount"
	case "RULE_MATCH_FIELD_TYPE":
		return "t.type"
	default:
		return "t.name"
	}
}

func textClause(col, op, pattern string) (string, []any, error) {
	switch op {
	case "RULE_MATCH_OPERATOR_CONTAINS":
		return `LOWER(` + col + `) LIKE '%' || LOWER(?) || '%' ESCAPE '\'`, []any{likeEscape(pattern)}, nil
	case "RULE_MATCH_OPERATOR_STARTS_WITH":
		return `LOWER(` + col + `) LIKE LOWER(?) || '%' ESCAPE '\'`, []any{likeEscape(pattern)}, nil
	case "RULE_MATCH_OPERATOR_ENDS_WITH":
		return `LOWER(` + col + `) LIKE '%' || LOWER(?) ESCAPE '\'`, []any{likeEscape(pattern)}, nil
	case "RULE_MATCH_OPERATOR_EQUALS":
		return `LOWER(` + col + `) = LOWER(?)`, []any{pattern}, nil
	case "RULE_MATCH_OPERATOR_GREATER_THAN":
		return col + ` > ?`, []any{pattern}, nil
	case "RULE_MATCH_OPERATOR_LESS_THAN":
		return col + ` < ?`, []any{pattern}, nil
	case "RULE_MATCH_OPERATOR_REGEX":
		return `regexp(?, ` + col + `)`, []any{pattern}, nil
	default:
		return "", nil, fmt.Errorf("unsupported operator %s", op)
	}
}

func numericClause(col, op, pattern string) (string, []any, error) {
	v, err := strconv.ParseFloat(pattern, 64)
	if err != nil {
		return "", nil, fmt.Errorf("invalid numeric pattern %q", pattern)
	}
	switch op {
	case "RULE_MATCH_OPERATOR_EQUALS":
		return col + ` = ?`, []any{v}, nil
	case "RULE_MATCH_OPERATOR_GREATER_THAN":
		return col + ` > ?`, []any{v}, nil
	case "RULE_MATCH_OPERATOR_LESS_THAN":
		return col + ` < ?`, []any{v}, nil
	default:
		return "", nil, fmt.Errorf("operator %s not supported for numeric field", op)
	}
}

// likeEscape escapes LIKE wildcards so the pattern is matched literally.
func likeEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
