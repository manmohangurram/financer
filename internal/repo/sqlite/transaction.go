package sqlite

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mohan9182/financer/internal/repo"
	tim "github.com/mohan9182/financer/internal/time"
)

// TransactionRepo implements repo.TransactionRepo over SQLite.
type TransactionRepo struct {
	db *sql.DB
}

func NewTransactionRepo(db *sql.DB) *TransactionRepo {
	return &TransactionRepo{db: db}
}

// refreshAccounts recomputes each touched account's balance and credit/debit
// totals from the transactions themselves, inside the caller's transaction.
// Recomputing rather than applying deltas means a retry cannot double-count.
func refreshAccounts(tx *sql.Tx, userID string, accountIDs map[string]bool) error {
	ids := make([]string, 0, len(accountIDs))
	for id := range accountIDs {
		if id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		if _, err := tx.Exec(`UPDATE accounts SET
				balance = ROUND(totals.balance, 2),
				total_credit = ROUND(totals.credit, 2),
				total_debit = ROUND(totals.debit, 2)
			FROM (
				SELECT COALESCE(SUM(CASE WHEN t.type = 'CREDIT' THEN t.amount ELSE -t.amount END), 0) AS balance,
				       COALESCE(SUM(CASE WHEN t.type = 'CREDIT' THEN t.amount ELSE 0 END), 0) AS credit,
				       COALESCE(SUM(CASE WHEN t.type = 'DEBIT' THEN t.amount ELSE 0 END), 0) AS debit
				  FROM transactions t WHERE t.account_id = ?
			) AS totals
			WHERE accounts.id = ? AND accounts.user_id = ?`, id, id, userID); err != nil {
			return err
		}
	}
	return nil
}

// accountsOf maps transaction ids to the account each is booked to.
func accountsOf(tx *sql.Tx, userID string, ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+1)
	args = append(args, userID)
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := tx.Query("SELECT id, account_id FROM transactions WHERE user_id = ? AND id IN ("+placeholders+")", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, accountID string
		if err := rows.Scan(&id, &accountID); err != nil {
			return nil, err
		}
		out[id] = accountID
	}
	return out, rows.Err()
}

func (r *TransactionRepo) GetForTransfer(userID, id string) (string, float64, string, bool, error) {
	var typ, accountID string
	var amount float64
	err := r.db.QueryRow(
		"SELECT type, amount, account_id FROM transactions WHERE id = ? AND user_id = ?", id, userID,
	).Scan(&typ, &amount, &accountID)
	if err == sql.ErrNoRows {
		return "", 0, "", false, nil
	}
	if err != nil {
		return "", 0, "", false, err
	}
	return typ, amount, accountID, true, nil
}

func (r *TransactionRepo) GetFull(userID, id string) (*repo.TransactionRow, error) {
	row := r.db.QueryRow(
		`SELECT t.id, t.name, t.clean_name, t.amount, t.type, t.occurred_at, t.account_id, t.created_at, t.external_id, t.ref_no
		 FROM transactions t WHERE t.id = ? AND t.user_id = ?`, id, userID,
	)
	txn, err := scanTransaction(row)
	if err != nil || txn == nil {
		return txn, err
	}
	linked, err := r.IsTransferLinked(userID, id)
	if err != nil {
		return nil, err
	}
	txn.TransferLinked = linked
	return txn, nil
}

// ExistingTransactionKeys lists the stored (date, type, amount) keys for an
// account, used to warn before importing duplicates.
func (r *TransactionRepo) ExistingTransactionKeys(userID, accountID, from, to string) ([]repo.TransactionKey, error) {
	rows, err := r.db.Query(
		`SELECT substr(occurred_at, 1, 10), type, amount
		 FROM transactions
		 WHERE user_id = ? AND account_id = ? AND substr(occurred_at, 1, 10) BETWEEN ? AND ?`,
		userID, accountID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repo.TransactionKey
	for rows.Next() {
		var k repo.TransactionKey
		if err := rows.Scan(&k.Date, &k.Type, &k.Amount); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (r *TransactionRepo) FindTransferCounterpart(userID, accountID, typ string, amount float64, around string) (*repo.TransactionRow, error) {
	aroundStr := around
	if len(aroundStr) > 19 {
		aroundStr = aroundStr[:19]
	}
	row := r.db.QueryRow(
		`SELECT t.id, t.name, t.clean_name, t.amount, t.type, t.occurred_at, t.account_id, t.created_at, NULL AS external_id, t.ref_no
		 FROM transactions t
		 WHERE t.user_id = ? AND t.account_id = ? AND t.type = ? AND t.amount = ?
		   AND t.occurred_at >= datetime(?, '-3 days') AND t.occurred_at <= datetime(?, '+3 days')
		   AND NOT EXISTS (SELECT 1 FROM transfer_links l WHERE l.debit_transaction_id = t.id OR l.credit_transaction_id = t.id)
		 ORDER BY ABS(CAST(strftime('%s', t.occurred_at) AS INTEGER) - CAST(strftime('%s', ?) AS INTEGER))
		 LIMIT 1`,
		userID, accountID, typ, amount, aroundStr, aroundStr, aroundStr,
	)
	return scanTransaction(row)
}

func (r *TransactionRepo) IsTransferLinked(userID, txnID string) (bool, error) {
	var one int
	err := r.db.QueryRow(
		"SELECT 1 FROM transfer_links WHERE debit_transaction_id = ? OR credit_transaction_id = ? LIMIT 1",
		txnID, txnID,
	).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *TransactionRepo) Create(userID string, inputs []repo.CreateTransactionInput) (*repo.CreateOutcome, error) {
	outcome := &repo.CreateOutcome{Inserted: map[string]bool{}}
	if len(inputs) == 0 {
		return outcome, nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	touched := map[string]bool{}
	for _, input := range inputs {
		t := input.Txn
		res, err := tx.Exec(
			`INSERT OR IGNORE INTO transactions (id, user_id, name, clean_name, amount, type, occurred_at, account_id, created_at, external_id, ref_no)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			t.ID, userID, t.Name, t.CleanName, t.Amount, t.Type, t.OccurredAt, t.AccountID, t.CreatedAt, t.ExternalID, t.RefNo,
		)
		if err != nil {
			outcome.Errors = append(outcome.Errors, fmt.Sprintf("failed to create %s", t.ID))
			continue
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			continue
		}
		outcome.Inserted[t.ID] = true
		touched[t.AccountID] = true
		for _, cat := range input.CategoryIDs {
			if _, err := tx.Exec(
				"INSERT OR IGNORE INTO transaction_categories (transaction_id, category_id) VALUES (?, ?)",
				t.ID, cat,
			); err != nil {
				outcome.Errors = append(outcome.Errors, fmt.Sprintf("failed to link category %s to %s", cat, t.ID))
			}
		}
	}
	if err := refreshAccounts(tx, userID, touched); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return outcome, nil
}

func (r *TransactionRepo) Update(userID string, inputs []repo.UpdateTransactionInput) ([]string, error) {
	var errs []string
	if len(inputs) == 0 {
		return errs, nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	ids := make([]string, 0, len(inputs))
	for _, in := range inputs {
		ids = append(ids, in.Txn.ID)
	}
	before, err := accountsOf(tx, userID, ids)
	if err != nil {
		return nil, err
	}
	touched := map[string]bool{}
	for _, input := range inputs {
		t := input.Txn
		res, err := tx.Exec(
			`UPDATE transactions SET
				name = COALESCE(NULLIF(?, ''), name),
				clean_name = CASE WHEN ? != '' THEN NULL ELSE clean_name END,
				amount = CASE WHEN ? != 0 THEN ? ELSE amount END,
				type = CASE WHEN ? != '' THEN ? ELSE type END,
				occurred_at = COALESCE(NULLIF(?, ''), occurred_at),
				account_id = COALESCE(NULLIF(?, ''), account_id),
				ref_no = COALESCE(NULLIF(?, ''), ref_no)
			 WHERE id = ? AND user_id = ?`,
			t.Name, t.Name, t.Amount, t.Amount, t.Type, t.Type, t.OccurredAt, t.AccountID, t.RefNo, t.ID, userID,
		)
		if err != nil {
			errs = append(errs, fmt.Sprintf("failed to update %s", t.ID))
			continue
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			errs = append(errs, fmt.Sprintf("transaction %s not found", t.ID))
			continue
		}
		oldAccount := before[t.ID]
		newAccount := oldAccount
		if t.AccountID != "" {
			newAccount = t.AccountID
		}
		touched[oldAccount] = true
		touched[newAccount] = true
		if _, err := tx.Exec("DELETE FROM transaction_categories WHERE transaction_id = ?", t.ID); err != nil {
			errs = append(errs, fmt.Sprintf("failed to clear categories for %s", t.ID))
			continue
		}
		for _, cat := range input.CategoryIDs {
			if _, err := tx.Exec(
				"INSERT OR IGNORE INTO transaction_categories (transaction_id, category_id) VALUES (?, ?)",
				t.ID, cat,
			); err != nil {
				errs = append(errs, fmt.Sprintf("failed to link category %s to %s", cat, t.ID))
			}
		}
	}
	if err := refreshAccounts(tx, userID, touched); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return errs, nil
}

func (r *TransactionRepo) ApplyRuleResult(userID, id string, cleanName *string, categoryIDs []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE transactions SET clean_name = ? WHERE id = ? AND user_id = ?", cleanName, id, userID); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM transaction_categories WHERE transaction_id = ?", id); err != nil {
		return err
	}
	for _, cat := range categoryIDs {
		if _, err := tx.Exec("INSERT OR IGNORE INTO transaction_categories (transaction_id, category_id) VALUES (?, ?)", id, cat); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *TransactionRepo) Delete(userID string, ids []string) ([]string, error) {
	var errs []string
	if len(ids) == 0 {
		return errs, nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	before, err := accountsOf(tx, userID, ids)
	if err != nil {
		return nil, err
	}
	touched := map[string]bool{}
	for _, id := range ids {
		res, err := tx.Exec("DELETE FROM transactions WHERE id = ? AND user_id = ?", id, userID)
		if err != nil {
			errs = append(errs, fmt.Sprintf("failed to delete %s", id))
			continue
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			errs = append(errs, fmt.Sprintf("transaction %s not found", id))
			continue
		}
		if accountID, ok := before[id]; ok {
			touched[accountID] = true
		}
	}
	if err := refreshAccounts(tx, userID, touched); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return errs, nil
}

func (r *TransactionRepo) List(userID string, f *repo.TransactionListFilter) (*repo.ListTransactionResult, error) {
	whereSQL, filterArgs := buildTxnWhere(f)

	query := `SELECT t.id, t.name, t.clean_name, t.amount, t.type, t.occurred_at, t.account_id, t.created_at, t.external_id, t.ref_no,
	                 COALESCE(l.id, ''),
	                 (SELECT GROUP_CONCAT(tc.category_id) FROM transaction_categories tc WHERE tc.transaction_id = t.id)
	          FROM transactions t
	          LEFT JOIN transfer_links l ON l.debit_transaction_id = t.id OR l.credit_transaction_id = t.id `
	query += whereSQL

	args := make([]any, 0, len(filterArgs)+1)
	args = append(args, userID)
	for _, a := range filterArgs {
		args = append(args, a)
	}

	countSQL := "SELECT COUNT(*) FROM transactions t " + whereSQL
	var total int64
	if err := r.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}

	if f.SortBy == "" {
		if f.PageToken != "" {
			if cur := decodeTransactionCursor(f.PageToken); cur != nil {
				query += " AND (t.occurred_at < ? OR (t.occurred_at = ? AND t.id < ?))"
				args = append(args, cur.OccurredAt, cur.OccurredAt, cur.ID)
			}
		}
		query += " ORDER BY t.occurred_at DESC, t.id DESC"
		if f.PageSize > 0 {
			query += fmt.Sprintf(" LIMIT %d", f.PageSize+1)
		}
	} else {
		primary := "DEBIT"
		if f.SortBy == "credit" {
			primary = "CREDIT"
		}
		dir := "DESC"
		if f.SortDir == "asc" {
			dir = "ASC"
		}
		groupExpr := fmt.Sprintf("CASE WHEN t.type = '%s' THEN 0 ELSE 1 END", primary)
		valueExpr := fmt.Sprintf("CASE WHEN t.type = '%s' THEN t.amount ELSE -t.amount END", primary)
		if cur := decodeSortCursor(f.PageToken); cur != nil {
			cmp := "<"
			if dir == "ASC" {
				cmp = ">"
			}
			query += fmt.Sprintf(" AND (%s > ? OR (%s = ? AND (%s %s ? OR (%s = ? AND t.id > ?))))",
				groupExpr, groupExpr, valueExpr, cmp, valueExpr)
			args = append(args, cur.Group, cur.Group, cur.Value, cur.Value, cur.ID)
		}
		query += fmt.Sprintf(" ORDER BY %s, %s %s, t.id ASC", groupExpr, valueExpr, dir)
		if f.PageSize > 0 {
			if f.Offset > 0 && f.PageToken == "" {
				query += fmt.Sprintf(" LIMIT %d OFFSET %d", f.PageSize, f.Offset)
			} else {
				query += fmt.Sprintf(" LIMIT %d", f.PageSize+1)
			}
		}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []repo.ListRow
	for rows.Next() {
		var txn repo.TransactionRow
		var cleanName, externalID, linkID, catGroup sql.NullString
		if err := rows.Scan(
			&txn.ID, &txn.Name, &cleanName, &txn.Amount, &txn.Type, &txn.OccurredAt,
			&txn.AccountID, &txn.CreatedAt, &externalID, &txn.RefNo, &linkID, &catGroup,
		); err != nil {
			return nil, err
		}
		if cleanName.Valid {
			txn.CleanName = &cleanName.String
		}
		if externalID.Valid {
			txn.ExternalID = &externalID.String
		}
		row := repo.ListRow{Txn: &txn, LinkID: linkID.String}
		if catGroup.Valid && catGroup.String != "" {
			row.CategoryIDs = strings.Split(catGroup.String, ",")
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var nextToken string
	if f.SortBy == "" {
		if f.PageSize > 0 && int64(len(items)) > f.PageSize {
			items = items[:f.PageSize]
			last := items[len(items)-1]
			nextToken = encodeTransactionCursor(last.Txn.OccurredAt, last.Txn.ID)
		}
	} else if f.PageToken != "" || f.Offset == 0 {
		if f.PageSize > 0 && int64(len(items)) > f.PageSize {
			items = items[:f.PageSize]
			last := items[len(items)-1]
			primary := "DEBIT"
			if f.SortBy == "credit" {
				primary = "CREDIT"
			}
			group := int64(1)
			value := -last.Txn.Amount
			if last.Txn.Type == primary {
				group = 0
				value = last.Txn.Amount
			}
			nextToken = encodeSortCursor(group, value, last.Txn.ID)
		}
	}
	return &repo.ListTransactionResult{Rows: items, NextPageToken: nextToken, TotalCount: total}, nil
}

func (r *TransactionRepo) SpendingRows(userID string, f *repo.SpendingFilter) ([]*repo.SpendingTxn, error) {
	rangeSQL := ""
	var rangeArgs []any
	if f.From != "" {
		rangeSQL += " AND t.occurred_at >= ?"
		rangeArgs = append(rangeArgs, f.From)
	}
	if f.To != "" {
		rangeSQL += " AND t.occurred_at < ?"
		rangeArgs = append(rangeArgs, f.To)
	}
	if f.AccountID != "" {
		rangeSQL += " AND t.account_id = ?"
		rangeArgs = append(rangeArgs, f.AccountID)
	}

	query := `SELECT t.id, t.occurred_at, t.amount, t.type, c.id, c.name
	          FROM transactions t
	          LEFT JOIN transaction_categories tc ON tc.transaction_id = t.id
	          LEFT JOIN categories c ON c.id = tc.category_id
	          LEFT JOIN transfer_links tl ON tl.debit_transaction_id = t.id OR tl.credit_transaction_id = t.id
	          LEFT JOIN accounts a_self ON a_self.id = t.account_id
	          LEFT JOIN accounts a_other ON a_other.id = CASE WHEN tl.debit_transaction_id = t.id THEN tl.credit_transaction_id ELSE tl.debit_transaction_id END
	          WHERE t.user_id = ? AND (tl.id IS NULL OR a_self.type IN ('CREDIT_CARD','LOAN') OR a_other.type IN ('CREDIT_CARD','LOAN'))
	          ` + rangeSQL

	args := make([]any, 0, len(rangeArgs)+1)
	args = append(args, userID)
	args = append(args, rangeArgs...)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*repo.SpendingTxn
	for rows.Next() {
		var s repo.SpendingTxn
		var catID, catName sql.NullString
		if err := rows.Scan(&s.ID, &s.OccurredAt, &s.Amount, &s.Type, &catID, &catName); err != nil {
			return nil, err
		}
		if catID.Valid {
			s.CategoryID = &catID.String
		}
		if catName.Valid {
			s.CategoryName = &catName.String
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}

func (r *TransactionRepo) CreateLinks(userID string, links [][2]string) ([]string, error) {
	var errs []string
	if len(links) == 0 {
		return errs, nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for _, link := range links {
		if _, err := tx.Exec(
			`INSERT INTO transfer_links (id, user_id, debit_transaction_id, credit_transaction_id, created_at)
			 VALUES (?, ?, ?, ?, ?)`,
			uuid.New().String(), userID, link[0], link[1], tim.NowGoTS(),
		); err != nil {
			errs = append(errs, fmt.Sprintf("failed to link transfer %s/%s", link[0], link[1]))
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return errs, nil
}

func (r *TransactionRepo) DeleteLinks(userID string, ids []string) ([]string, error) {
	var errs []string
	if len(ids) == 0 {
		return errs, nil
	}
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for _, id := range ids {
		res, err := tx.Exec("DELETE FROM transfer_links WHERE id = ? AND user_id = ?", id, userID)
		if err != nil {
			errs = append(errs, fmt.Sprintf("failed to delete transfer link %s", id))
			continue
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			errs = append(errs, fmt.Sprintf("transfer link %s not found", id))
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return errs, nil
}

func buildTxnWhere(f *repo.TransactionListFilter) (string, []any) {
	query := "WHERE t.user_id = ?"
	var args []any
	if f.AccountID != "" {
		query += " AND t.account_id = ?"
		args = append(args, f.AccountID)
	}
	if len(f.CategoryIDs) > 0 {
		ph := strings.TrimSuffix(strings.Repeat("?,", len(f.CategoryIDs)), ",")
		query += " AND EXISTS (SELECT 1 FROM transaction_categories tc WHERE tc.transaction_id = t.id AND tc.category_id IN (" + ph + "))"
		for _, id := range f.CategoryIDs {
			args = append(args, id)
		}
	}
	if f.Type != nil {
		query += " AND t.type = ?"
		args = append(args, *f.Type)
	}
	if f.DateFrom != "" {
		query += " AND t.occurred_at >= ?"
		args = append(args, f.DateFrom)
	}
	if f.DateTo != "" {
		query += " AND t.occurred_at < ?"
		args = append(args, f.DateTo)
	}
	if f.MinAmount > 0 {
		query += " AND t.amount >= ?"
		args = append(args, f.MinAmount)
	}
	if f.MaxAmount > 0 {
		query += " AND t.amount <= ?"
		args = append(args, f.MaxAmount)
	}
	if len(f.Names) > 0 {
		clauses := make([]string, 0, len(f.Names))
		for _, n := range f.Names {
			clauses = append(clauses, `LOWER(t.name) LIKE '%' || LOWER(?) || '%' ESCAPE '\'`)
			args = append(args, likeEscape(n))
		}
		query += " AND (" + strings.Join(clauses, " OR ") + ")"
	}
	return query, args
}

func likeEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	return strings.ReplaceAll(s, "_", `\_`)
}

func scanTransaction(row *sql.Row) (*repo.TransactionRow, error) {
	var t repo.TransactionRow
	var cleanName, externalID sql.NullString
	err := row.Scan(&t.ID, &t.Name, &cleanName, &t.Amount, &t.Type, &t.OccurredAt, &t.AccountID, &t.CreatedAt, &externalID, &t.RefNo)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if cleanName.Valid {
		t.CleanName = &cleanName.String
	}
	if externalID.Valid {
		t.ExternalID = &externalID.String
	}
	return &t, nil
}

type transactionCursor struct {
	OccurredAt string `json:"OccurredAt"`
	ID         string `json:"ID"`
}

type txnSortCursor struct {
	Group int64   `json:"g"`
	Value float64 `json:"v"`
	ID    string  `json:"id"`
}

func encodeSortCursor(group int64, value float64, id string) string {
	payload, _ := json.Marshal(txnSortCursor{Group: group, Value: value, ID: id})
	return base64.URLEncoding.EncodeToString(payload)
}

func decodeSortCursor(token string) *txnSortCursor {
	if token == "" {
		return nil
	}
	raw, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return nil
	}
	var payload txnSortCursor
	if err := json.Unmarshal(raw, &payload); err != nil || payload.ID == "" {
		return nil
	}
	return &payload
}

func encodeTransactionCursor(occurredAt, id string) string {
	payload, _ := json.Marshal(transactionCursor{OccurredAt: tim.TSRFC3339(occurredAt), ID: id})
	return base64.URLEncoding.EncodeToString(payload)
}

func decodeTransactionCursor(token string) *transactionCursor {
	raw, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return nil
	}
	var payload struct {
		OccurredAt string `json:"OccurredAt"`
		ID         string `json:"ID"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.ID == "" {
		return nil
	}
	stored := payload.OccurredAt
	if t, err := parseRFC3339UTC(payload.OccurredAt); err == nil {
		stored = tim.FromUTC(t)
	}
	return &transactionCursor{OccurredAt: stored, ID: payload.ID}
}

func parseRFC3339UTC(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}
