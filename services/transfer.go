package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mohan9182/financer/auth"
	"github.com/mohan9182/financer/api"
	"github.com/mohan9182/financer/repository"
)

type TransferService struct {
	txnRepo     *repository.TransactionRepository
	linkRepo    *repository.TransferRepository
	accountRepo *repository.AccountRepository
}

func NewTransferService(txnRepo *repository.TransactionRepository, linkRepo *repository.TransferRepository, accountRepo *repository.AccountRepository) *TransferService {
	return &TransferService{txnRepo: txnRepo, linkRepo: linkRepo, accountRepo: accountRepo}
}

func accountDisplayName(a *api.AccountResponse) string {
	if n := a.AccountNickname; n != "" {
		return n
	}
	return a.BankName
}

// CreateCounterpart creates the missing side of a transfer for an existing
// transaction and links it. If a matching counterpart already exists it is
// linked instead (find-or-create via resolveTransfer).
func (s *TransferService) CreateCounterpart(ctx context.Context, msg *api.CreateCounterpartRequest) (*api.CreateTransferResponse, error) {
	userID, _ := ctx.Value(auth.UserIDKey).(string)
	txn, err := s.txnRepo.GetByID(ctx, msg.TransactionId)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	if txn == nil {
		return nil, NotFound("transaction %s not found", msg.TransactionId)
	}
	if msg.ToAccountId == "" || msg.ToAccountId == txn.AccountId {
		return nil, BadRequest("to account must differ from the source account")
	}

	linked, _, counterpartID, err := resolveTransfer(ctx, userID, txn, msg.ToAccountId, s.txnRepo, s.linkRepo, s.accountRepo)
	if err != nil {
		return nil, err
	}
	if !linked {
		return nil, BadRequest("source transaction is already linked")
	}

	resp := &api.CreateTransferResponse{}
	if txn.Type == api.TransactionType_DEBIT {
		resp.DebitTransactionId = txn.Id
		resp.CreditTransactionId = counterpartID
	} else {
		resp.CreditTransactionId = txn.Id
		resp.DebitTransactionId = counterpartID
	}
	return resp, nil
}

// resolveTransfer finds or creates the counterpart of txn in targetAccountID and
// links them. Same-account and already-linked transactions are no-ops. Returns
// whether a link was created (linked), whether a new transaction was created
// (created), and the counterpart transaction id.
func resolveTransfer(ctx context.Context, userID string, txn *api.TransactionResponse, targetAccountID string, txnRepo *repository.TransactionRepository, linkRepo *repository.TransferRepository, accountRepo *repository.AccountRepository) (linked, created bool, counterpartID string, err error) {
	if txn.AccountId == targetAccountID || txn.LinkedTransferId != "" {
		return false, false, "", nil
	}
	opposite := api.TransactionType_CREDIT
	if txn.Type == api.TransactionType_CREDIT {
		opposite = api.TransactionType_DEBIT
	}

	cand, err := txnRepo.FindTransferCounterpart(ctx, userID, targetAccountID, opposite, txn.Amount, txn.OccurredAt)
	if err != nil {
		return false, false, "", err
	}
	if cand != nil {
		debitID, creditID := txn.Id, cand.Id
		if txn.Type == api.TransactionType_CREDIT {
			debitID, creditID = cand.Id, txn.Id
		}
		if _, errs := linkRepo.Create(ctx, userID, []repository.LinkTransferInput{{DebitTransactionID: debitID, CreditTransactionID: creditID}}); len(errs) > 0 {
			return false, false, "", errs[0]
		}
		return true, false, cand.Id, nil
	}

	// No unlinked counterpart found. The other side may have just been linked
	// in this run while we were holding a stale row — check before creating a
	// duplicate.
	linkedNow, err := txnRepo.IsTransferLinked(ctx, userID, txn.Id)
	if err != nil {
		return false, false, "", err
	}
	if linkedNow {
		return false, false, "", nil
	}

	srcAcc, err := accountRepo.GetByID(ctx, txn.AccountId)
	if err != nil || srcAcc == nil {
		return false, false, "", fmt.Errorf("source account not found")
	}
	counterTxn := &api.TransactionResponse{
		Id: uuid.New().String(), Name: "Transfer from " + accountDisplayName(srcAcc), Amount: txn.Amount,
		Type: opposite, AccountId: targetAccountID, OccurredAt: txn.OccurredAt, CreatedAt: time.Now().UTC(),
	}
	if errs := txnRepo.Create(ctx, userID, []repository.CreateTransactionInput{{Txn: counterTxn}}); len(errs) > 0 {
		return false, false, "", errs[0]
	}
	debitID, creditID := txn.Id, counterTxn.Id
	if txn.Type == api.TransactionType_CREDIT {
		debitID, creditID = counterTxn.Id, txn.Id
	}
	if _, errs := linkRepo.Create(ctx, userID, []repository.LinkTransferInput{{DebitTransactionID: debitID, CreditTransactionID: creditID}}); len(errs) > 0 {
		return false, false, "", errs[0]
	}
	return true, true, counterTxn.Id, nil
}

func (s *TransferService) LinkTransfers(ctx context.Context, msg *api.LinkTransfersRequest) (*api.BulkOperationResponse, error) {
	links := msg.Links
	if len(links) == 0 {
		return nil, BadRequest("no links provided")
	}
	userID, _ := ctx.Value(auth.UserIDKey).(string)

	var inputs []repository.LinkTransferInput
	var preErrs []string

	for i, link := range links {
		if link.DebitTransactionId == "" || link.CreditTransactionId == "" {
			preErrs = append(preErrs, fmt.Sprintf("link %d: both debit and credit transaction IDs are required", i))
			continue
		}
		if link.DebitTransactionId == link.CreditTransactionId {
			preErrs = append(preErrs, fmt.Sprintf("link %d: debit and credit transactions must be different", i))
			continue
		}

		debitType, _, debitAccount, err := s.txnRepo.GetByIDForTransfer(ctx, link.DebitTransactionId)
		if err != nil {
			preErrs = append(preErrs, fmt.Sprintf("debit transaction %s not found", link.DebitTransactionId))
			continue
		}

		creditType, _, creditAccount, err := s.txnRepo.GetByIDForTransfer(ctx, link.CreditTransactionId)
		if err != nil {
			preErrs = append(preErrs, fmt.Sprintf("credit transaction %s not found", link.CreditTransactionId))
			continue
		}

		if debitType != int(api.TransactionType_DEBIT) {
			preErrs = append(preErrs, fmt.Sprintf("transaction %s is not a DEBIT", link.DebitTransactionId))
			continue
		}
		if creditType != int(api.TransactionType_CREDIT) {
			preErrs = append(preErrs, fmt.Sprintf("transaction %s is not a CREDIT", link.CreditTransactionId))
			continue
		}
		if debitAccount == creditAccount {
			preErrs = append(preErrs, fmt.Sprintf("link %d: debit and credit must be from different accounts", i))
			continue
		}

		debitLinked, _ := s.linkRepo.IsTransactionLinked(ctx, link.DebitTransactionId)
		creditLinked, _ := s.linkRepo.IsTransactionLinked(ctx, link.CreditTransactionId)
		if debitLinked || creditLinked {
			preErrs = append(preErrs, fmt.Sprintf("one or both transactions in link %d are already linked", i))
			continue
		}

		inputs = append(inputs, repository.LinkTransferInput{
			DebitTransactionID:  link.DebitTransactionId,
			CreditTransactionID: link.CreditTransactionId,
		})
	}

	_, errs := s.linkRepo.Create(ctx, userID, inputs)
	var failedIDs []string
	for _, e := range preErrs {
		failedIDs = append(failedIDs, e)
	}
	for _, e := range errs {
		failedIDs = append(failedIDs, e.Error())
	}

	if len(failedIDs) > 0 {
		return &api.BulkOperationResponse{
			Success:   false,
			Message:   "some transfers failed",
			FailedIds: failedIDs,
		}, nil
	}

	return &api.BulkOperationResponse{
		Success: true,
		Message: "transfers linked successfully",
	}, nil
}

func (s *TransferService) UnlinkTransfers(ctx context.Context, msg *api.UnlinkTransfersRequest) (*api.BulkOperationResponse, error) {
	ids := msg.Ids
	if len(ids) == 0 {
		return nil, BadRequest("no ids provided")
	}

	errs := s.linkRepo.Delete(ctx, ids)
	if len(errs) > 0 {
		return &api.BulkOperationResponse{
			Success:   false,
			Message:   "some unlinks failed",
			FailedIds: errStrings(errs),
		}, nil
	}

	return &api.BulkOperationResponse{
		Success: true,
		Message: "transfer links removed successfully",
	}, nil
}