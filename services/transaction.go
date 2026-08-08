package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mohan9182/financer/auth"
	"github.com/mohan9182/financer/api"
	"github.com/mohan9182/financer/logx"
	"github.com/mohan9182/financer/repository"
)

type TransactionService struct {
	txnRepo      *repository.TransactionRepository
	accRepo      *repository.AccountRepository
	ruleSvc      *RuleService
	transferRule *TransferRuleService
}

func NewTransactionService(txnRepo *repository.TransactionRepository, accRepo *repository.AccountRepository, ruleSvc *RuleService, transferRule *TransferRuleService) *TransactionService {
	return &TransactionService{txnRepo: txnRepo, accRepo: accRepo, ruleSvc: ruleSvc, transferRule: transferRule}
}

func txnDelta(txn *api.TransactionResponse) float32 {
	if txn.Type == api.TransactionType_CREDIT {
		return txn.Amount
	}
	return -txn.Amount
}

// txnTotals splits a transaction into its credit/debit contribution for the
// cached per-account totals (credit txn → credit, debit txn → debit).
func txnTotals(txn *api.TransactionResponse) (credit, debit float32) {
	if txn.Type == api.TransactionType_CREDIT {
		return txn.Amount, 0
	}
	return 0, txn.Amount
}

// Dashboard returns the user's balance and income/expense sums. Portfolio and
// account/investment lists are assembled by the handler from their services.
func (s *TransactionService) Dashboard(ctx context.Context) (*api.DashboardResponse, error) {
	userID, _ := ctx.Value(auth.UserIDKey).(string)
	balance, err := s.accRepo.SumBalance(ctx, userID)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	credit, debit, err := s.accRepo.SumTotals(ctx, userID)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	return &api.DashboardResponse{TotalBalance: balance, TotalIncome: credit, TotalExpenses: debit}, nil
}

// Spending returns day/month spending buckets and per-category debit/credit/
// net for a range, computed in SQL (transfer exclusion: non-debt transfers
// don't count; debt-account transfers do).
func (s *TransactionService) Spending(ctx context.Context, msg *api.SpendingRequest) (*api.SpendingResponse, error) {
	userID := ctx.Value(auth.UserIDKey).(string)
	now := time.Now().UTC()

	gran := "day"
	var from, to time.Time
	switch msg.Range {
	case "7D":
		from, to = now.AddDate(0, 0, -7), now
	case "1M":
		from, to = now.AddDate(0, -1, 0), now
	case "6M":
		from, to, gran = now.AddDate(0, -6, 0), now, "month"
	case "1Y":
		from, to, gran = now.AddDate(0, -12, 0), now, "month"
	}
	if msg.From != "" || msg.To != "" {
		from, _ = time.Parse("2006-01-02", msg.From)
		to, _ = time.Parse("2006-01-02", msg.To)
		if !to.IsZero() {
			to = to.Add(24 * time.Hour) // inclusive end-of-day
		}
		if !from.IsZero() && !to.IsZero() {
			if to.Sub(from) > 366*24*time.Hour {
				return nil, BadRequest("custom range must be at most 1 year")
			}
			if int(to.Sub(from).Hours()/24) > 30 {
				gran = "month"
			}
		}
	}
	f := repository.SpendingFilter{Granularity: gran, From: from, To: to, AccountID: msg.AccountId}

	rows, err := s.txnRepo.SpendingBuckets(ctx, userID, f)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	cats, err := s.txnRepo.SpendingCategories(ctx, userID, f)
	if err != nil {
		return nil, ServerError("%v", err)
	}

	buckets := make([]*api.SpendingBucket, 0, len(rows))
	for _, r := range rows {
		buckets = append(buckets, &api.SpendingBucket{Key: r.Key, Label: bucketLabel(r.Key, gran), Amount: r.Amount})
	}
	for _, c := range cats {
		c.Net = c.Debit - c.Credit
	}
	return &api.SpendingResponse{Buckets: buckets, Categories: cats}, nil
}

func bucketLabel(key, gran string) string {
	layout := "2006-01-02"
	if gran == "month" {
		layout = "2006-01"
	}
	t, err := time.Parse(layout, key)
	if err != nil {
		return key
	}
	if gran == "month" {
		return t.Format("Jan 06")
	}
	return fmt.Sprintf("%d %s", t.Day(), t.Format("Jan"))
}

func (s *TransactionService) ListTransactions(ctx context.Context, msg *api.ListTransactionsRequest) (*api.ListTransactionsResponse, error) {
	userID, _ := ctx.Value(auth.UserIDKey).(string)

	f := repository.TxnListFilter{
		AccountID: msg.AccountId, CategoryIDs: msg.CategoryId, TxnType: msg.Type,
		DateFrom: msg.DateFrom, DateTo: msg.DateTo, MinAmount: msg.MinAmount, MaxAmount: msg.MaxAmount,
		Name: msg.Name, NameMatch: msg.NameMatch, PageSize: msg.PageSize, PageToken: msg.PageToken,
		SortBy: msg.SortBy, SortDir: msg.SortDir, Offset: msg.Offset,
	}
	result, err := s.txnRepo.List(ctx, userID, f)
	if err != nil {
		return nil, ServerError("%v", err)
	}

	if s.ruleSvc != nil {
		if err := s.ruleSvc.Overlay(ctx, result.Transactions); err != nil {
			return nil, ServerError("%v", err)
		}
	}

	return &api.ListTransactionsResponse{
		Transactions:  result.Transactions,
		NextPageToken: result.NextPageToken,
		TotalCount:    result.TotalCount,
	}, nil
}

func (s *TransactionService) CreateTransactions(ctx context.Context, msg *api.CreateTransactionsRequest) (*api.BulkOperationResponse, error) {
	if len(msg.Transactions) > 1000 {
		return nil, BadRequest("too many transactions in one request (max 1000)")
	}
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
			Id:          uuid.New().String(),
			Name:        t.Name,
			Amount:      round2f(t.Amount),
			Type:        t.Type,
			OccurredAt:  occurredAt,
			AccountId:   t.AccountId,
			CreatedAt:   now,
			CategoryIds: t.CategoryIds,
			ExternalId:  t.ExternalId,
		})
	}

	inputs := make([]repository.CreateTransactionInput, len(txns))
	for i, txn := range txns {
		inputs[i] = repository.CreateTransactionInput{Txn: txn, CategoryIDs: txn.CategoryIds}
	}
	inserted, errs := s.txnRepo.Create(ctx, userID, inputs)

	var insertedTxns []*api.TransactionResponse
	skipped := 0
	for _, txn := range txns {
		if !inserted[txn.Id] {
			skipped++
			continue
		}
		insertedTxns = append(insertedTxns, txn)
		s.accRepo.UpdateBalance(ctx, txn.AccountId, txnDelta(txn))
		credit, debit := txnTotals(txn)
		s.accRepo.ApplyTotals(ctx, txn.AccountId, credit, debit)
	}

	if s.transferRule != nil && len(insertedTxns) > 0 {
		if _, _, err := s.transferRule.ApplyToTransactions(ctx, insertedTxns); err != nil {
			logx.Error("transfer rule apply failed", "err", err)
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return &api.BulkOperationResponse{
			Success:   false,
			Message:   "some transactions failed",
			FailedIds: errStrings(errs),
			Skipped:   int32(skipped),
		}, nil
	}
	return &api.BulkOperationResponse{
		Success: true,
		Message: "transactions created successfully",
		Skipped: int32(skipped),
	}, nil
}

func (s *TransactionService) UpdateTransactions(ctx context.Context, msg *api.UpdateTransactionsRequest) (*api.BulkOperationResponse, error) {
	if len(msg.Transactions) > 1000 {
		return nil, BadRequest("too many transactions in one request (max 1000)")
	}
	var txns []*api.TransactionResponse
	for _, t := range msg.Transactions {
		txn := &api.TransactionResponse{
			Id:          t.Id,
			Name:        t.Name,
			Amount:      round2f(t.Amount),
			Type:        t.Type,
			AccountId:   t.AccountId,
			CategoryIds: t.CategoryIds,
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
	oldTxns, err := s.txnRepo.GetByIDBatch(ctx, ids)
	if err != nil {
		logx.With("count", len(ids)).Error("failed to preload transactions before update (balances may drift)", "err", err)
	}
	oldMap := make(map[string]*api.TransactionResponse, len(oldTxns))
	for _, t := range oldTxns {
		oldMap[t.Id] = t
	}

	inputs := make([]repository.UpdateTransactionInput, len(txns))
	for i := range txns {
		inputs[i] = repository.UpdateTransactionInput{Txn: txns[i], CategoryIDs: txns[i].CategoryIds}
	}
	errs := s.txnRepo.Update(ctx, inputs)

	// reverse old, apply new per account (balance + cached credit/debit totals)
	type acctDeltas struct{ balance, credit, debit float32 }
	deltas := make(map[string]*acctDeltas)
	accDelta := func(id string, d float32, c float32, dd float32) {
		a := deltas[id]
		if a == nil {
			a = &acctDeltas{}
			deltas[id] = a
		}
		a.balance += d
		a.credit += c
		a.debit += dd
	}

	for _, txn := range txns {
		old := oldMap[txn.Id]
		if old == nil {
			continue
		}
		accID := txn.AccountId
		if accID == "" {
			accID = old.AccountId
		}
		oc, od := txnTotals(old)
		accDelta(old.AccountId, -txnDelta(old), -oc, -od)
		nc, nd := txnTotals(txn)
		accDelta(accID, txnDelta(txn), nc, nd)
	}

	for accID, a := range deltas {
		if a.balance != 0 {
			s.accRepo.UpdateBalance(ctx, accID, a.balance)
		}
		if a.credit != 0 || a.debit != 0 {
			s.accRepo.ApplyTotals(ctx, accID, a.credit, a.debit)
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

	oldTxns, err := s.txnRepo.GetByIDBatch(ctx, ids)
	if err != nil {
		logx.With("count", len(ids)).Error("failed to preload transactions before delete (balances may drift)", "err", err)
	}
	errs := s.txnRepo.Delete(ctx, ids)

	// Reverse balance + cached credit/debit totals per account (all txns in
	// the batch, not just the first per account).
	type acctDeltas struct{ balance, credit, debit float32 }
	deltas := make(map[string]*acctDeltas)
	for _, txn := range oldTxns {
		a := deltas[txn.AccountId]
		if a == nil {
			a = &acctDeltas{}
			deltas[txn.AccountId] = a
		}
		c, d := txnTotals(txn)
		a.balance -= txnDelta(txn)
		a.credit -= c
		a.debit -= d
	}
	for accID, a := range deltas {
		if a.balance != 0 {
			s.accRepo.UpdateBalance(ctx, accID, a.balance)
		}
		if a.credit != 0 || a.debit != 0 {
			s.accRepo.ApplyTotals(ctx, accID, a.credit, a.debit)
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
