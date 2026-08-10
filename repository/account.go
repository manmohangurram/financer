package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mohan9182/financer/api"
)

type AccountRepository struct {
	*BaseRepository
}

func NewAccountRepository(base *BaseRepository) *AccountRepository {
	return &AccountRepository{BaseRepository: base}
}

func scanAccount(row scannable) (*api.AccountResponse, error) {
	var acc api.AccountResponse
	var createdAt time.Time
	var accType int
	err := row.Scan(&acc.Id, &acc.BankName, &acc.AccountNickname, &acc.Balance, &accType, &createdAt)
	if err != nil {
		return nil, err
	}
	acc.AccountType = api.AccountType(accType)
	acc.CreatedAt = createdAt
	return &acc, nil
}

func (r *AccountRepository) Create(ctx context.Context, userID, bankName string, nickname string, accountType api.AccountType) (*api.AccountResponse, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	if accountType == api.AccountType_ACCOUNT_TYPE_UNSPECIFIED {
		accountType = api.AccountType_ACCOUNT_TYPE_CHECKING
	}

	_, err := r.ExecContext(ctx,
		`INSERT INTO accounts (id, user_id, bank_name, account_nickname, balance, type, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, userID, bankName, nickname, 0.0, int(accountType), now,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting account: %w", err)
	}

	return &api.AccountResponse{
		Id:              id,
		BankName:        bankName,
		AccountNickname: nickname,
		Balance:         0.0,
		AccountType:     accountType,
		CreatedAt:       now,
	}, nil
}

func (r *AccountRepository) GetByID(ctx context.Context, id string) (*api.AccountResponse, error) {
	return QueryOne(ctx, r.readDB,
		`SELECT id, bank_name, account_nickname, balance, type, created_at FROM accounts WHERE id = ?`,
		scanAccount, id,
	)
}

func (r *AccountRepository) Update(ctx context.Context, id, bankName string, nickname string, accountType api.AccountType) (*api.AccountResponse, error) {
	result, err := r.ExecContext(ctx,
		`UPDATE accounts SET
			bank_name = COALESCE(NULLIF(?, ''), bank_name),
			account_nickname = COALESCE(NULLIF(?, ''), account_nickname),
			type = CASE WHEN ? != 0 THEN ? ELSE type END
		 WHERE id = ?`,
		bankName, nickname, int(accountType), int(accountType), id,
	)
	if err != nil {
		return nil, fmt.Errorf("updating account: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, nil
	}

	return r.GetByID(ctx, id)
}

func (r *AccountRepository) Delete(ctx context.Context, id string) (bool, error) {
	result, err := r.ExecContext(ctx, `DELETE FROM accounts WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("deleting account: %w", err)
	}

	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

func (r *AccountRepository) List(ctx context.Context, userID string) ([]*api.AccountResponse, error) {
	return QueryAll(ctx, r.readDB,
		`SELECT id, bank_name, account_nickname, balance, type, created_at FROM accounts WHERE user_id = ? ORDER BY created_at DESC`,
		scanAccount, userID,
	)
}

// UpdateBalance applies a signed delta to an account's stored balance.
func (r *AccountRepository) UpdateBalance(ctx context.Context, accountID string, delta float32) error {
	_, err := r.ExecContext(ctx,
		`UPDATE accounts SET balance = ROUND(COALESCE(balance, 0) + ?, 2) WHERE id = ?`,
		delta, accountID,
	)
	return err
}

// SumBalance returns the total stored balance across a user's accounts.
func (r *AccountRepository) SumBalance(ctx context.Context, userID string) (float32, error) {
	var total float32
	if err := r.readDB.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE user_id = ?`, userID,
	).Scan(&total); err != nil {
		return 0, fmt.Errorf("summing balances: %w", err)
	}
	return total, nil
}

// ApplyTotals applies signed deltas to an account's cached credit/debit totals.
func (r *AccountRepository) ApplyTotals(ctx context.Context, accountID string, credit, debit float32) error {
	_, err := r.ExecContext(ctx,
		`UPDATE accounts SET
			total_credit = ROUND(COALESCE(total_credit, 0) + ?, 2),
			total_debit  = ROUND(COALESCE(total_debit, 0) + ?, 2)
		 WHERE id = ?`,
		credit, debit, accountID,
	)
	return err
}

// SumTotals returns the cached total credits and debits across a user's
// accounts (no transactions scan).
func (r *AccountRepository) SumTotals(ctx context.Context, userID string) (credit, debit float32, err error) {
	err = r.readDB.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(total_credit), 0), COALESCE(SUM(total_debit), 0) FROM accounts WHERE user_id = ?`, userID,
	).Scan(&credit, &debit)
	return credit, debit, err
}
