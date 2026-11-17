// Package sqlite is the SQLite implementation of repo.AccountRepo and repo.CategoryRepo.
package sqlite

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	cursorlib "github.com/mohan9182/financer/internal/cursor"
	"github.com/mohan9182/financer/internal/repo"
)

type AccountRepo struct {
	db *sql.DB
}

func NewAccountRepo(db *sql.DB) *AccountRepo {
	return &AccountRepo{db: db}
}

func (r *AccountRepo) Create(userID, bankName, nickname, accountType, endingNumbers string) (*repo.AccountRow, error) {
	id := uuid.New().String()
	now := nowGoTS()
	_, err := r.db.Exec(
		"INSERT INTO accounts (id, user_id, bank_name, nickname, balance, type, ending_numbers, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		id, userID, bankName, nickname, 0.0, accountType, endingNumbers, now,
	)
	if err != nil {
		return nil, err
	}
	return &repo.AccountRow{
		ID:            id,
		BankName:      bankName,
		Nickname:      nickname,
		Balance:       0,
		AccountType:   accountType,
		EndingNumbers: &endingNumbers,
		CreatedAt:     now,
	}, nil
}

func (r *AccountRepo) GetByID(userID, id string) (*repo.AccountRow, error) {
	return r.scanAccount(r.db.QueryRow(
		"SELECT id, bank_name, nickname, balance, type, ending_numbers, created_at FROM accounts WHERE id = ? AND user_id = ?", id, userID,
	))
}

func (r *AccountRepo) Update(userID, id, bankName, nickname, accountType, endingNumbers string) (*repo.AccountRow, error) {
	res, err := r.db.Exec(
		"UPDATE accounts SET bank_name = COALESCE(NULLIF(?, ''), bank_name), nickname = COALESCE(NULLIF(?, ''), nickname), type = ?, ending_numbers = COALESCE(NULLIF(?, ''), ending_numbers) WHERE id = ? AND user_id = ?",
		bankName, nickname, accountType, endingNumbers, id, userID,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, nil
	}
	return r.GetByID(userID, id)
}

func (r *AccountRepo) Delete(userID, id string) (bool, error) {
	res, err := r.db.Exec("DELETE FROM accounts WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *AccountRepo) List(userID string) ([]*repo.AccountRow, error) {
	rows, err := r.db.Query(
		"SELECT id, bank_name, nickname, balance, type, ending_numbers, created_at FROM accounts WHERE user_id = ? ORDER BY created_at DESC", userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*repo.AccountRow
	for rows.Next() {
		a, err := scanAccountRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListPage returns one keyset page of accounts plus the next token.
func (r *AccountRepo) ListPage(userID string, limit int64, cursor *repo.AccountCursor) ([]*repo.AccountRow, string, error) {
	if limit < 1 {
		limit = 100
	}
	query := "SELECT id, bank_name, nickname, balance, type, ending_numbers, CAST(created_at AS TEXT) FROM accounts WHERE user_id = ?"
	args := []any{userID}
	if cursor != nil {
		query += " AND (created_at < ? OR (created_at = ? AND id < ?))"
		args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	query += " ORDER BY created_at DESC, id DESC LIMIT ?"
	args = append(args, limit+1)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []*repo.AccountRow
	for rows.Next() {
		a, err := scanAccountRow(rows)
		if err != nil {
			return nil, "", err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if int64(len(out)) > limit {
		out = out[:limit]
		last := out[len(out)-1]
		next = cursorlib.Encode(last.CreatedAt, last.ID)
	}
	return out, next, nil
}

func (r *AccountRepo) SumBalance(userID string) (float64, error) {
	var total float64
	if err := r.db.QueryRow("SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE user_id = ?", userID).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *AccountRepo) SumTotals(userID string) (credit, debit float64, err error) {
	err = r.db.QueryRow("SELECT COALESCE(SUM(total_credit), 0), COALESCE(SUM(total_debit), 0) FROM accounts WHERE user_id = ?", userID).Scan(&credit, &debit)
	return
}

func (r *AccountRepo) scanAccount(row *sql.Row) (*repo.AccountRow, error) {
	var a repo.AccountRow
	var ending sql.NullString
	if err := row.Scan(&a.ID, &a.BankName, &a.Nickname, &a.Balance, &a.AccountType, &ending, &a.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if ending.Valid {
		a.EndingNumbers = &ending.String
	}
	return &a, nil
}

func scanAccountRow(rows *sql.Rows) (*repo.AccountRow, error) {
	var a repo.AccountRow
	var ending sql.NullString
	if err := rows.Scan(&a.ID, &a.BankName, &a.Nickname, &a.Balance, &a.AccountType, &ending, &a.CreatedAt); err != nil {
		return nil, err
	}
	if ending.Valid {
		a.EndingNumbers = &ending.String
	}
	return &a, nil
}

// --- categories ---

type CategoryRepo struct {
	db *sql.DB
}

func NewCategoryRepo(db *sql.DB) *CategoryRepo {
	return &CategoryRepo{db: db}
}

func (r *CategoryRepo) Create(userID string, inputs []repo.CategoryCreateInput) ([]*repo.CategoryRow, error) {
	var out []*repo.CategoryRow
	for _, input := range inputs {
		id := uuid.New().String()
		now := nowGoTS()
		if _, err := r.db.Exec(
			"INSERT INTO categories (id, user_id, name, monthly_budget, created_at) VALUES (?, ?, ?, ?, ?)",
			id, userID, input.Name, input.MonthlyBudget, now,
		); err != nil {
			return nil, err
		}
		out = append(out, &repo.CategoryRow{ID: id, Name: input.Name, MonthlyBudget: input.MonthlyBudget, CreatedAt: now})
	}
	return out, nil
}

func (r *CategoryRepo) GetByID(userID, id string) (*repo.CategoryRow, error) {
	return r.scanCategory(r.db.QueryRow(
		"SELECT id, name, monthly_budget, created_at FROM categories WHERE id = ? AND user_id = ? LIMIT 1", id, userID,
	))
}

func (r *CategoryRepo) Update(userID string, inputs []repo.CategoryUpdateInput) ([]string, error) {
	var errs []string
	for _, input := range inputs {
		res, err := r.db.Exec(
			"UPDATE categories SET name = COALESCE(NULLIF(?, ''), name), monthly_budget = COALESCE(?, monthly_budget) WHERE id = ? AND user_id = ?",
			input.Name, input.MonthlyBudget, input.ID, userID,
		)
		if err != nil {
			errs = append(errs, fmt.Sprintf("category %s: %v", input.ID, err))
			continue
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			errs = append(errs, fmt.Sprintf("category %s not found", input.ID))
		}
	}
	return errs, nil
}

func (r *CategoryRepo) Delete(userID string, ids []string) ([]string, error) {
	var errs []string
	for _, id := range ids {
		res, err := r.db.Exec("DELETE FROM categories WHERE id = ? AND user_id = ?", id, userID)
		if err != nil {
			errs = append(errs, fmt.Sprintf("category %s: %v", id, err))
			continue
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			errs = append(errs, fmt.Sprintf("category %s not found", id))
		}
	}
	return errs, nil
}

func (r *CategoryRepo) List(userID string, pageSize int64, pageToken string) (*repo.ListCategoriesResult, error) {
	cursor := decodeCatCursor(pageToken)
	query := "SELECT id, name, monthly_budget, created_at FROM categories WHERE user_id = ?"
	args := []any{userID}
	if cursor != nil {
		query += " AND (name > ? OR (name = ? AND id > ?))"
		args = append(args, cursor.Name, cursor.Name, cursor.ID)
	}
	query += " ORDER BY name ASC, id ASC"
	if pageSize > 0 {
		query += " LIMIT ?"
		args = append(args, pageSize+1)
	}
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var categories []*repo.CategoryRow
	for rows.Next() {
		c, err := scanCategoryRow(rows)
		if err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var nextToken string
	if pageSize > 0 && int64(len(categories)) > pageSize {
		categories = categories[:pageSize]
		if last := categories[len(categories)-1]; last != nil {
			nextToken = encodeCatCursor(last.Name, last.ID)
		}
	}
	return &repo.ListCategoriesResult{Categories: categories, NextPageToken: nextToken}, nil
}

func (r *CategoryRepo) scanCategory(row *sql.Row) (*repo.CategoryRow, error) {
	var c repo.CategoryRow
	if err := row.Scan(&c.ID, &c.Name, &c.MonthlyBudget, &c.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func scanCategoryRow(rows *sql.Rows) (*repo.CategoryRow, error) {
	var c repo.CategoryRow
	if err := rows.Scan(&c.ID, &c.Name, &c.MonthlyBudget, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

// SetBudget stores the category's recurring monthly limit (0 = no budget).
func (r *CategoryRepo) SetBudget(userID, id string, budget float64) (bool, error) {
	res, err := r.db.Exec(
		"UPDATE categories SET monthly_budget = ? WHERE id = ? AND user_id = ?",
		budget, id, userID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// --- cursor ---

type catCursor struct {
	Name string `json:"n"`
	ID   string `json:"id"`
}

func encodeCatCursor(name, id string) string {
	payload, _ := json.Marshal(catCursor{Name: name, ID: id})
	return base64.URLEncoding.EncodeToString(payload)
}

func decodeCatCursor(token string) *catCursor {
	if token == "" {
		return nil
	}
	raw, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return nil
	}
	var c catCursor
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil
	}
	return &c
}

var _ = strings.TrimSpace

func nowGoTS() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05 -0700 MST")
}
