package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mohan9182/financer/auth"
	"github.com/mohan9182/financer/api"
	"github.com/mohan9182/financer/repository"
)

type TransactionService struct {
	txnRepo *repository.TransactionRepository
	accRepo *repository.AccountRepository
}

func NewTransactionService(txnRepo *repository.TransactionRepository, accRepo *repository.AccountRepository) *TransactionService {
	return &TransactionService{txnRepo: txnRepo, accRepo: accRepo}
}

func txnDelta(txn *api.TransactionResponse) float32 {
	if txn.Type == api.TransactionType_CREDIT {
		return txn.Amount
	}
	return -txn.Amount
}

func (s *TransactionService) GetTransaction(ctx context.Context, msg *api.GetTransactionRequest) (*api.TransactionResponse, error) {
	txn, err := s.txnRepo.GetByID(ctx, msg.Id)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	if txn == nil {
		return nil, NotFound("transaction %s not found", msg.Id)
	}
	return txn, nil
}

func (s *TransactionService) ListTransactions(ctx context.Context, msg *api.ListTransactionsRequest) (*api.ListTransactionsResponse, error) {
	userID, _ := ctx.Value(auth.UserIDKey).(string)

	f := repository.TxnListFilter{
		AccountID: msg.AccountId, TxnType: msg.Type,
		DateFrom: msg.DateFrom, DateTo: msg.DateTo, MinAmount: msg.MinAmount, MaxAmount: msg.MaxAmount,
		Name: msg.Name, NameMatch: msg.NameMatch, PageSize: msg.PageSize, PageToken: msg.PageToken,
		SortBy: msg.SortBy, SortDir: msg.SortDir, Offset: msg.Offset,
	}
	result, err := s.txnRepo.List(ctx, userID, f)
	if err != nil {
		return nil, ServerError("%v", err)
	}

	count, _ := s.txnRepo.Count(ctx, userID, f)

	return &api.ListTransactionsResponse{
		Transactions:  result.Transactions,
		NextPageToken: result.NextPageToken,
		TotalCount:    count,
	}, nil
}

func (s *TransactionService) CreateTransactions(ctx context.Context, msg *api.CreateTransactionsRequest) (*api.BulkOperationResponse, error) {
	userID, _ := ctx.Value(auth.UserIDKey).(string)
	var txns []*api.TransactionResponse
	now := time.Now().UTC()
	for _, t := range msg.Transactions {
		if t.Name == "" || t.Amount <= 0 || t.AccountId == "" {
			continue
		}
		occurredAt := now
		if !t.OccurredAt.IsZero() {
			occurredAt = t.OccurredAt
		}
		txns = append(txns, &api.TransactionResponse{
			Id:         uuid.New().String(),
			Name:       t.Name,
			Amount:     round2f(t.Amount),
			Type:       t.Type,
			OccurredAt: occurredAt,
			AccountId:  t.AccountId,
			CreatedAt:  now,
		})
	}

	inputs := make([]repository.CreateTransactionInput, len(txns))
	for i, txn := range txns {
		inputs[i] = repository.CreateTransactionInput{Txn: txn}
	}
	errs := s.txnRepo.Create(ctx, userID, inputs)

	for _, txn := range txns {
		s.accRepo.UpdateBalance(ctx, txn.AccountId, txnDelta(txn))
	}

	if len(errs) > 0 {
		return &api.BulkOperationResponse{
			Success:   false,
			Message:   "some transactions failed",
			FailedIds: errStrings(errs),
		}, nil
	}
	return &api.BulkOperationResponse{
		Success: true,
		Message: "transactions created successfully",
	}, nil
}

func (s *TransactionService) UpdateTransactions(ctx context.Context, msg *api.UpdateTransactionsRequest) (*api.BulkOperationResponse, error) {
	var txns []*api.TransactionResponse
	for _, t := range msg.Transactions {
		txn := &api.TransactionResponse{
			Id:        t.Id,
			Name:      t.Name,
			Amount:    round2f(t.Amount),
			Type:      t.Type,
			AccountId: t.AccountId,
		}
		if !t.OccurredAt.IsZero() {
			txn.OccurredAt = t.OccurredAt
		}
		txns = append(txns, txn)
	}

	ids := make([]string, len(txns))
	for i, txn := range txns {
		ids[i] = txn.Id
	}
	oldTxns, _ := s.txnRepo.GetByIDBatch(ctx, ids)
	oldMap := make(map[string]*api.TransactionResponse, len(oldTxns))
	for _, t := range oldTxns {
		oldMap[t.Id] = t
	}

	inputs := make([]repository.UpdateTransactionInput, len(txns))
	for i := range txns {
		inputs[i] = repository.UpdateTransactionInput{Txn: txns[i]}
	}
	errs := s.txnRepo.Update(ctx, inputs)

	// reverse old, apply new per account
	type acctDelta struct{ old, new float32 }
	deltas := make(map[string]*acctDelta)

	for _, txn := range txns {
		old := oldMap[txn.Id]
		if old == nil {
			continue
		}
		d := deltas[old.AccountId]
		if d == nil {
			d = &acctDelta{}
			deltas[old.AccountId] = d
		}
		d.old += txnDelta(old)

		accID := txn.AccountId
		if accID == "" {
			accID = old.AccountId
		}
		d2 := deltas[accID]
		if d2 == nil {
			d2 = &acctDelta{}
			deltas[accID] = d2
		}
		d2.new += txnDelta(txn)
	}

	for accID, d := range deltas {
		delta := d.new - d.old
		if delta != 0 {
			s.accRepo.UpdateBalance(ctx, accID, delta)
		}
	}

	if len(errs) > 0 {
		return &api.BulkOperationResponse{
			Success:   false,
			Message:   "some updates failed",
			FailedIds: errStrings(errs),
		}, nil
	}
	return &api.BulkOperationResponse{
		Success: true,
		Message: "transactions updated successfully",
	}, nil
}

func (s *TransactionService) DeleteTransactions(ctx context.Context, msg *api.DeleteTransactionsRequest) (*api.BulkOperationResponse, error) {
	ids := msg.Ids
	if len(ids) == 0 {
		return nil, BadRequest("no ids provided")
	}

	oldTxns, _ := s.txnRepo.GetByIDBatch(ctx, ids)
	errs := s.txnRepo.Delete(ctx, ids)

	seen := make(map[string]bool)
	for _, txn := range oldTxns {
		if !seen[txn.AccountId] {
			s.accRepo.UpdateBalance(ctx, txn.AccountId, -txnDelta(txn))
			seen[txn.AccountId] = true
		}
	}

	if len(errs) > 0 {
		return &api.BulkOperationResponse{
			Success:   false,
			Message:   "some deletions failed",
			FailedIds: errStrings(errs),
		}, nil
	}
	return &api.BulkOperationResponse{
		Success: true,
		Message: "transactions deleted successfully",
	}, nil
}

func errStrings(errs []error) []string {
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, e.Error())
	}
	return out
}
