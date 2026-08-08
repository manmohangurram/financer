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
	Txn               *api.TransactionResponse
	CategoryIDs       []string
	ReplaceCategories bool
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

func (r *TransactionRepository) List(ctx context.Context, userID string, f TxnListFilter) (*ListTxnResult, error) {
	where, args := buildTxnWhere(f)
	args = append([]any{userID}, args...)

	query := `SELECT t.id, t.name, t.amount, t.type, t.occurred_at, t.account_id, t.created_at
			  FROM transactions t ` + where

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

	results, err := QueryAll(ctx, r.readDB, query, scanTransaction, args...)
	if err != nil {
		return nil, fmt.Errorf("listing transactions: %w", err)
	}

	var nextToken string
	if f.SortBy == "" && f.PageSize > 0 && int32(len(results)) > f.PageSize {
		results = results[:f.PageSize]
		last := results[len(results)-1]
		nextToken = encodeTxnCursor(TxnCursor{
			OccurredAt: last.OccurredAt,
			ID:         last.Id,
		})
	}

	for _, txn := range results {
		r.populateLink(ctx, txn)
	}
	r.populateCategories(ctx, results...)

	return &ListTxnResult{Transactions: results, NextPageToken: nextToken}, nil
}

func (r *TransactionRepository) Count(ctx context.Context, userID string, f TxnListFilter) (int32, error) {
	where, args := buildTxnWhere(f)
	args = append([]any{userID}, args...)
	var n int32
	if err := r.readDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM transactions t "+where, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("counting transactions: %w", err)
	}
	return n, nil
}

// populateLink attaches the transfer link id (if any) to a transaction.
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

// populateCategories fills CategoryIds for a batch of transactions with a
// single query.
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

func (r *TransactionRepository) Create(ctx context.Context, userID string, inputs []CreateTransactionInput) []error {
	if len(inputs) == 0 {
		return nil
	}

	var errors []error

	r.ExecInTx(ctx, func(tx *Tx) error {
		stmt, err := tx.PrepareContext(ctx,
			`INSERT INTO transactions (id, user_id, name, amount, type, occurred_at, account_id, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
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
			if _, err := stmt.ExecContext(ctx, txn.Id, userID, txn.Name, txn.Amount, int(txn.Type), txn.OccurredAt, txn.AccountId, txn.CreatedAt); err != nil {
				errors = append(errors, fmt.Errorf("failed to create %s: %w", txn.Id, err))
				continue
			}
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

	return errors
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
			if input.ReplaceCategories {
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
