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
		`INSERT INTO accounts (id, user_id, bank_name, account_nickname, balance, account_type, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
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
		`SELECT id, bank_name, account_nickname, balance, account_type, created_at FROM accounts WHERE id = ?`,
		scanAccount, id,
	)
}

func (r *AccountRepository) Update(ctx context.Context, id, bankName string, nickname string, accountType api.AccountType) (*api.AccountResponse, error) {
	result, err := r.ExecContext(ctx,
		`UPDATE accounts SET
			bank_name = COALESCE(NULLIF(?, ''), bank_name),
			account_nickname = COALESCE(NULLIF(?, ''), account_nickname),
			account_type = CASE WHEN ? != 0 THEN ? ELSE account_type END
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
		`SELECT id, bank_name, account_nickname, balance, account_type, created_at FROM accounts WHERE user_id = ? ORDER BY created_at DESC`,
		scanAccount, userID,
	)
}
