// Package transferservice implements transfer linking: pairing a debit and a
// credit transaction as the two sides of a transfer, and creating the missing
// counterpart when it does not exist yet.
package transferservice

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
	tim "github.com/mohan9182/financer/internal/time"
)

type Service struct {
	txns     repo.TransactionRepo
	accounts repo.AccountRepo
}

func New(txns repo.TransactionRepo, accounts repo.AccountRepo) *Service {
	return &Service{txns: txns, accounts: accounts}
}

// ResolveOutcome reports whether a counterpart was linked or created.
type ResolveOutcome struct {
	Linked        bool
	Created       bool
	CounterpartID string
}

// CreateCounterpart creates or links the missing side of a transfer for an
// existing transaction.
func (s *Service) CreateCounterpart(userID, txnID, toAccountID string) (*model.CreateTransferResp, error) {
	src, err := s.txns.GetFull(userID, txnID)
	if err != nil {
		return nil, err
	}
	if src == nil {
		return nil, errors.NotFound("transaction " + txnID + " not found")
	}
	if toAccountID == "" || toAccountID == src.AccountID {
		return nil, errors.BadRequest("to account must differ from the source account")
	}
	acct, err := s.accounts.GetByID(userID, toAccountID)
	if err != nil {
		return nil, err
	}
	if acct == nil {
		return nil, errors.BadRequest("account does not belong to user")
	}

	outcome, err := s.ResolveTransfer(userID, src, toAccountID)
	if err != nil {
		return nil, err
	}
	if !outcome.Linked {
		return nil, errors.BadRequest("source transaction is already linked")
	}
	if src.Type == string(model.TransactionTypeDebit) {
		return &model.CreateTransferResp{
			DebitTransactionID:  src.ID,
			CreditTransactionID: outcome.CounterpartID,
		}, nil
	}
	return &model.CreateTransferResp{
		DebitTransactionID:  outcome.CounterpartID,
		CreditTransactionID: src.ID,
	}, nil
}

// LinkTransfers validates and creates explicit debit/credit pairs.
func (s *Service) LinkTransfers(userID string, links [][2]string) (*model.WireBulk, error) {
	if len(links) == 0 {
		return nil, errors.BadRequest("no links provided")
	}
	var inputs [][2]string
	var failed []string
	for i, link := range links {
		debitID, creditID := link[0], link[1]
		if debitID == "" || creditID == "" {
			failed = append(failed, fmt.Sprintf("link %d: both debit and credit transaction IDs are required", i))
			continue
		}
		if debitID == creditID {
			failed = append(failed, fmt.Sprintf("link %d: debit and credit transactions must be different", i))
			continue
		}
		dt, _, da, ok, err := s.txns.GetForTransfer(userID, debitID)
		if err != nil {
			return nil, err
		}
		if !ok {
			failed = append(failed, "debit transaction "+debitID+" not found")
			continue
		}
		ct, _, ca, ok, err := s.txns.GetForTransfer(userID, creditID)
		if err != nil {
			return nil, err
		}
		if !ok {
			failed = append(failed, "credit transaction "+creditID+" not found")
			continue
		}
		if dt != string(model.TransactionTypeDebit) {
			failed = append(failed, "transaction "+debitID+" is not a DEBIT")
			continue
		}
		if ct != string(model.TransactionTypeCredit) {
			failed = append(failed, "transaction "+creditID+" is not a CREDIT")
			continue
		}
		if da == ca {
			failed = append(failed, fmt.Sprintf("link %d: debit and credit must be from different accounts", i))
			continue
		}
		debitLinked, err := s.txns.IsTransferLinked(userID, debitID)
		if err != nil {
			return nil, err
		}
		creditLinked, err := s.txns.IsTransferLinked(userID, creditID)
		if err != nil {
			return nil, err
		}
		if debitLinked || creditLinked {
			failed = append(failed, fmt.Sprintf("one or both transactions in link %d are already linked", i))
			continue
		}
		inputs = append(inputs, [2]string{debitID, creditID})
	}

	errs, err := s.txns.CreateLinks(userID, inputs)
	if err != nil {
		return nil, err
	}
	failed = append(failed, errs...)
	if len(failed) > 0 {
		return &model.WireBulk{Success: false, Message: "some transfers failed", FailedIDs: failed}, nil
	}
	return &model.WireBulk{Success: true, Message: "transfers linked successfully"}, nil
}

// UnlinkTransfers removes transfer links by id.
func (s *Service) UnlinkTransfers(userID string, ids []string) (*model.WireBulk, error) {
	if len(ids) == 0 {
		return nil, errors.BadRequest("no ids provided")
	}
	errs, err := s.txns.DeleteLinks(userID, ids)
	if err != nil {
		return nil, err
	}
	if len(errs) > 0 {
		return &model.WireBulk{Success: false, Message: "some unlinks failed", FailedIDs: errs}, nil
	}
	return &model.WireBulk{Success: true, Message: "transfer links removed successfully"}, nil
}

// ResolveTransfer finds or creates the counterpart of src in targetAccountID
// and links them. Same-account and already-linked transactions are no-ops.
func (s *Service) ResolveTransfer(userID string, src *repo.TransactionRow, targetAccountID string) (*ResolveOutcome, error) {
	if src.AccountID == targetAccountID || src.TransferLinked {
		return &ResolveOutcome{}, nil
	}
	opposite := string(model.TransactionTypeCredit)
	if src.Type == string(model.TransactionTypeCredit) {
		opposite = string(model.TransactionTypeDebit)
	}

	if cand, err := s.txns.FindTransferCounterpart(userID, targetAccountID, opposite, src.Amount, src.OccurredAt); err != nil {
		return nil, err
	} else if cand != nil {
		debitID, creditID := src.ID, cand.ID
		if src.Type == string(model.TransactionTypeCredit) {
			debitID, creditID = cand.ID, src.ID
		}
		errs, err := s.txns.CreateLinks(userID, [][2]string{{debitID, creditID}})
		if err != nil {
			return nil, err
		}
		if len(errs) > 0 {
			return nil, errors.Internal(errs[0])
		}
		return &ResolveOutcome{Linked: true, CounterpartID: cand.ID}, nil
	}

	// No unlinked counterpart found; the other side may have just been linked.
	linked, err := s.txns.IsTransferLinked(userID, src.ID)
	if err != nil {
		return nil, err
	}
	if linked {
		return &ResolveOutcome{}, nil
	}

	counterTxn := &repo.TransactionRow{
		ID:         uuid.New().String(),
		Name:       "Transfer from " + s.accountDisplay(userID, src.AccountID),
		Amount:     src.Amount,
		Type:       opposite,
		AccountID:  targetAccountID,
		OccurredAt: src.OccurredAt,
		CreatedAt:  tim.NowGoTS(),
	}
	outcome, err := s.txns.Create(userID, []repo.CreateTransactionInput{{Txn: counterTxn}})
	if err != nil {
		return nil, err
	}
	if len(outcome.Errors) > 0 {
		return nil, errors.Internal(outcome.Errors[0])
	}
	if !outcome.Inserted[counterTxn.ID] {
		return &ResolveOutcome{}, nil
	}

	debitID, creditID := src.ID, counterTxn.ID
	if src.Type == string(model.TransactionTypeCredit) {
		debitID, creditID = counterTxn.ID, src.ID
	}
	errs, err := s.txns.CreateLinks(userID, [][2]string{{debitID, creditID}})
	if err != nil {
		return nil, err
	}
	if len(errs) > 0 {
		return nil, errors.Internal(errs[0])
	}
	return &ResolveOutcome{Linked: true, Created: true, CounterpartID: counterTxn.ID}, nil
}

func (s *Service) accountDisplay(userID, accountID string) string {
	acct, err := s.accounts.GetByID(userID, accountID)
	if err != nil || acct == nil {
		return accountID
	}
	if acct.Nickname != "" {
		return acct.Nickname
	}
	return acct.BankName
}
