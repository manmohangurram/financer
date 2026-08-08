package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type TransferRepository struct {
	*BaseRepository
}

func NewTransferRepository(base *BaseRepository) *TransferRepository {
	return &TransferRepository{BaseRepository: base}
}

type LinkTransferInput struct {
	DebitTransactionID  string
	CreditTransactionID string
}

func (r *TransferRepository) Create(ctx context.Context, userID string, inputs []LinkTransferInput) []error {
	if len(inputs) == 0 {
		return nil
	}

	var errors []error

	r.ExecInTx(ctx, func(tx *Tx) error {
		for _, input := range inputs {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO transfer_links (id, user_id, debit_transaction_id, credit_transaction_id, created_at)
				 VALUES (?, ?, ?, ?, ?)`,
				uuid.New().String(), userID, input.DebitTransactionID, input.CreditTransactionID, time.Now().UTC(),
			); err != nil {
				errors = append(errors, fmt.Errorf("failed to link transfer %s/%s: %w", input.DebitTransactionID, input.CreditTransactionID, err))
				continue
			}
		}
		if len(errors) > 0 {
			return fmt.Errorf("some transfers failed")
		}
		return nil
	})

	return errors
}

func (r *TransferRepository) Delete(ctx context.Context, ids []string) []error {
	if len(ids) == 0 {
		return nil
	}

	var errors []error

	r.ExecInTx(ctx, func(tx *Tx) error {
		for _, id := range ids {
			if _, err := tx.ExecContext(ctx, `DELETE FROM transfer_links WHERE id = ?`, id); err != nil {
				errors = append(errors, fmt.Errorf("failed to delete transfer link %s: %w", id, err))
			}
		}
		if len(errors) > 0 {
			return fmt.Errorf("some deletions failed")
		}
		return nil
	})

	return errors
}

func (r *TransferRepository) IsTransactionLinked(ctx context.Context, txnID string) (bool, error) {
	var existing string
	err := r.readDB.QueryRowContext(ctx,
		`SELECT id FROM transfer_links WHERE debit_transaction_id = ? OR credit_transaction_id = ?`,
		txnID, txnID,
	).Scan(&existing)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
