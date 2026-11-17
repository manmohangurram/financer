// Package transactionservice implements transaction business logic: ownership
// checks, bulk create/update/delete, list filtering, and write-time rule
// application (clean_name + categories are snapshotted, never the raw name).
package transactionservice

import (
	"log/slog"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
	tim "github.com/mohan9182/financer/internal/time"
)

const maxBulk = 1000

// Request is one transaction ready for create/update. OccurredAt is a stored
// timestamp; empty means "now" on create and "unchanged" on update.
type Request struct {
	ID          string
	Name        string
	Amount      float64
	Type        string
	OccurredAt  string
	AccountID   string
	CategoryIDs []string
	ExternalID  *string
}

// RuleApplier applies write-time rule actions to transaction views.
type RuleApplier interface {
	Overlay(userID string, views []repo.TransactionView) error
}

// TransferApplier applies transfer rules (SET_TRANSFER_ACCOUNT) to new rows.
type TransferApplier interface {
	ApplyToTransactions(userID string, txns []*repo.TransactionRow) (linked, created int64, err error)
}

type Service struct {
	txns       repo.TransactionRepo
	accounts   repo.AccountRepo
	categories repo.CategoryRepo
	rule       RuleApplier
	transfer   TransferApplier
}

func New(txns repo.TransactionRepo, accounts repo.AccountRepo, categories repo.CategoryRepo) *Service {
	return &Service{txns: txns, accounts: accounts, categories: categories}
}

// WithRule enables write-time rule application.
func (s *Service) WithRule(rule RuleApplier) *Service {
	s.rule = rule
	return s
}

// WithTransferRule enables auto-linking of transfer counterparts on create.
func (s *Service) WithTransferRule(t TransferApplier) *Service {
	s.transfer = t
	return s
}

func (s *Service) List(userID string, f *repo.TransactionListFilter) (*repo.ListTransactionResult, error) {
	if f.DateFrom != "" {
		d := f.DateFrom
		v, ok := tim.LocalDayStart(d)
		if !ok {
			return nil, errors.BadRequest("invalid dateFrom " + d)
		}
		f.DateFrom = v
	}
	if f.DateTo != "" {
		d := f.DateTo
		v, ok := tim.LocalDayEnd(d)
		if !ok {
			return nil, errors.BadRequest("invalid dateTo " + d)
		}
		f.DateTo = v
	}
	return s.txns.List(userID, f)
}

func (s *Service) verifyOwnership(userID string, t *Request) error {
	acct, err := s.accounts.GetByID(userID, t.AccountID)
	if err != nil {
		return err
	}
	if acct == nil {
		return errors.BadRequest("account does not belong to user")
	}
	for _, cat := range t.CategoryIDs {
		c, err := s.categories.GetByID(userID, cat)
		if err != nil {
			return err
		}
		if c == nil {
			return errors.BadRequest("category does not belong to user")
		}
	}
	return nil
}

func (s *Service) applyRules(userID string, inputs []repo.CreateTransactionInput) error {
	if s.rule == nil {
		return nil
	}
	views := make([]repo.TransactionView, len(inputs))
	for i := range inputs {
		t := inputs[i].Txn
		views[i] = repo.TransactionView{
			Name:        t.Name,
			Amount:      t.Amount,
			Type:        t.Type,
			AccountID:   t.AccountID,
			CategoryIDs: inputs[i].CategoryIDs,
			OccurredAt:  t.OccurredAt,
		}
	}
	if err := s.rule.Overlay(userID, views); err != nil {
		return err
	}
	for i := range inputs {
		if views[i].Name != inputs[i].Txn.Name {
			name := views[i].Name
			inputs[i].Txn.CleanName = &name
		}
		inputs[i].CategoryIDs = views[i].CategoryIDs
	}
	return nil
}

func (s *Service) Create(userID string, reqs []Request) (*model.WireBulk, error) {
	if len(reqs) > maxBulk {
		return nil, errors.BadRequest("too many transactions in one request (max 1000)")
	}
	now := tim.NowGoTS()
	inputs := make([]repo.CreateTransactionInput, 0, len(reqs))
	txns := make([]*repo.TransactionRow, 0, len(reqs))
	for _, t := range reqs {
		if t.Name == "" {
			return nil, errors.BadRequest("invalid transaction: name is required")
		}
		if t.Amount <= 0 {
			return nil, errors.BadRequest("invalid transaction: amount must be greater than zero")
		}
		if t.AccountID == "" {
			return nil, errors.BadRequest("invalid transaction: account_id is required")
		}
		if err := s.verifyOwnership(userID, &t); err != nil {
			return nil, err
		}
		occurredAt := t.OccurredAt
		if occurredAt == "" {
			occurredAt = now
		}
		txn := &repo.TransactionRow{
			ID:         t.ID,
			Name:       t.Name,
			Amount:     round2(t.Amount),
			Type:       t.Type,
			OccurredAt: occurredAt,
			AccountID:  t.AccountID,
			CreatedAt:  now,
			ExternalID: t.ExternalID,
		}
		inputs = append(inputs, repo.CreateTransactionInput{Txn: txn, CategoryIDs: t.CategoryIDs})
		txns = append(txns, txn)
	}

	if err := s.applyRules(userID, inputs); err != nil {
		return nil, err
	}
	outcome, err := s.txns.Create(userID, inputs)
	if err != nil {
		return nil, err
	}

	var inserted []*repo.TransactionRow
	var skipped int64
	for _, t := range txns {
		if !outcome.Inserted[t.ID] {
			skipped++
			continue
		}
		inserted = append(inserted, t)
	}
	if s.transfer != nil && len(inserted) > 0 {
		if _, _, err := s.transfer.ApplyToTransactions(userID, inserted); err != nil {
			slog.Warn("transfer rule application failed", "err", err)
		}
	}
	if len(outcome.Errors) > 0 {
		return &model.WireBulk{
			Success:   false,
			Message:   "some transactions failed",
			FailedIDs: outcome.Errors,
			Skipped:   skipped,
		}, nil
	}
	return &model.WireBulk{Success: true, Message: "transactions created successfully", Skipped: skipped}, nil
}

func (s *Service) Update(userID string, reqs []Request) (*model.WireBulk, error) {
	if len(reqs) > maxBulk {
		return nil, errors.BadRequest("too many transactions in one request (max 1000)")
	}
	for _, t := range reqs {
		if t.AccountID != "" {
			acct, err := s.accounts.GetByID(userID, t.AccountID)
			if err != nil {
				return nil, err
			}
			if acct == nil {
				return nil, errors.BadRequest("account does not belong to user")
			}
		}
		for _, cat := range t.CategoryIDs {
			c, err := s.categories.GetByID(userID, cat)
			if err != nil {
				return nil, err
			}
			if c == nil {
				return nil, errors.BadRequest("category does not belong to user")
			}
		}
	}
	inputs := make([]repo.UpdateTransactionInput, 0, len(reqs))
	for _, t := range reqs {
		inputs = append(inputs, repo.UpdateTransactionInput{
			Txn: &repo.TransactionRow{
				ID:         t.ID,
				Name:       t.Name,
				Amount:     round2(t.Amount),
				Type:       t.Type,
				OccurredAt: t.OccurredAt,
				AccountID:  t.AccountID,
			},
			CategoryIDs: t.CategoryIDs,
		})
	}
	errs, err := s.txns.Update(userID, inputs)
	if err != nil {
		return nil, err
	}
	if len(errs) > 0 {
		return &model.WireBulk{Success: false, Message: "some updates failed", FailedIDs: errs}, nil
	}
	return &model.WireBulk{Success: true, Message: "transactions updated successfully"}, nil
}

func (s *Service) Delete(userID string, ids []string) (*model.WireBulk, error) {
	if len(ids) == 0 {
		return nil, errors.BadRequest("no ids provided")
	}
	errs, err := s.txns.Delete(userID, ids)
	if err != nil {
		return nil, err
	}
	if len(errs) > 0 {
		return &model.WireBulk{Success: false, Message: "some deletions failed", FailedIDs: errs}, nil
	}
	return &model.WireBulk{Success: true, Message: "transactions deleted successfully"}, nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// Dashboard is the headline summary (total balance, income, expenses).
type Dashboard struct {
	TotalBalance  float64
	TotalIncome   float64
	TotalExpenses float64
}

// Dashboard reads the cached account totals.
func (s *Service) Dashboard(userID string) (*Dashboard, error) {
	balance, err := s.accounts.SumBalance(userID)
	if err != nil {
		return nil, err
	}
	credit, debit, err := s.accounts.SumTotals(userID)
	if err != nil {
		return nil, err
	}
	return &Dashboard{TotalBalance: balance, TotalIncome: credit, TotalExpenses: debit}, nil
}

// SpendingBucket is one day/month bucket of debit spending.
type SpendingBucket struct {
	Key    string
	Label  string
	Amount float64
}

// SpendingCategory aggregates debit/credit per category for the range.
type SpendingCategory struct {
	ID     string
	Name   string
	Debit  float64
	Credit float64
	Net    float64
}

// SpendingResult is the spending endpoint response payload.
type SpendingResult struct {
	Buckets    []SpendingBucket
	Categories []SpendingCategory
}

// Spending aggregates debit spending into day/month buckets and per-category
// totals. Buckets use the configured timezone (a DST-correct local bucket
// cannot be expressed in SQL, since the UTC offset varies per row).
func (s *Service) Spending(userID, rang, from, to, accountID string) (*SpendingResult, error) {
	now := time.Now().UTC()
	gran := "day"
	var fromTS, toTS string

	switch rang {
	case "7D":
		fromTS = tim.FromUTC(now.AddDate(0, 0, -7))
		toTS = tim.FromUTC(now)
	case "1M":
		fromTS = tim.FromUTC(addMonthsClamped(now, -1))
		toTS = tim.FromUTC(now)
	case "6M":
		fromTS = tim.FromUTC(addMonthsClamped(now, -6))
		toTS = tim.FromUTC(now)
		gran = "month"
	case "1Y":
		fromTS = tim.FromUTC(addMonthsClamped(now, -12))
		toTS = tim.FromUTC(now)
		gran = "month"
	}

	if from != "" || to != "" {
		f, fok := tim.LocalDateStartUTC(from)
		t, tok := tim.LocalDateStartUTC(to)
		if fok {
			fromTS = tim.FromUTC(f)
		} else {
			fromTS = ""
		}
		// `to` is inclusive, so the end bound is local midnight of the next day.
		if tok {
			toTS = tim.FromUTC(t.AddDate(0, 0, 1))
		} else {
			toTS = ""
		}
		if fok && tok && int(t.Sub(f).Hours()/24) > 30 {
			gran = "month"
		}
	}

	rows, err := s.txns.SpendingRows(userID, &repo.SpendingFilter{
		From:      fromTS,
		To:        toTS,
		AccountID: accountID,
	})
	if err != nil {
		return nil, err
	}

	bucketOrder := make([]string, 0)
	bucketTotals := make(map[string]float64)
	catOrder := make([]string, 0)
	catTotals := make(map[string]*SpendingCategory)
	bucketed := make(map[string]bool)

	for _, r := range rows {
		isDebit := r.Type == string(model.TransactionTypeDebit)
		if isDebit && !bucketed[r.ID] {
			bucketed[r.ID] = true
			key := tim.LocalKey(r.OccurredAt, gran)
			if _, ok := bucketTotals[key]; !ok {
				bucketOrder = append(bucketOrder, key)
			}
			bucketTotals[key] += r.Amount
		}
		id, name := "__uncategorized__", "Uncategorized"
		if r.CategoryID != nil && r.CategoryName != nil {
			id, name = *r.CategoryID, *r.CategoryName
		}
		agg, ok := catTotals[id]
		if !ok {
			agg = &SpendingCategory{ID: id, Name: name}
			catTotals[id] = agg
			catOrder = append(catOrder, id)
		}
		if isDebit {
			agg.Debit += r.Amount
		} else {
			agg.Credit += r.Amount
		}
	}

	sort.Slice(bucketOrder, func(i, j int) bool { return bucketOrder[i] < bucketOrder[j] })
	buckets := make([]SpendingBucket, 0, len(bucketOrder))
	for _, key := range bucketOrder {
		buckets = append(buckets, SpendingBucket{
			Key:    key,
			Label:  bucketLabel(key, gran),
			Amount: bucketTotals[key],
		})
	}

	categories := make([]SpendingCategory, 0, len(catOrder))
	for _, id := range catOrder {
		c := *catTotals[id]
		c.Net = c.Debit - c.Credit
		categories = append(categories, c)
	}
	sort.SliceStable(categories, func(i, j int) bool { return categories[i].Debit > categories[j].Debit })

	return &SpendingResult{Buckets: buckets, Categories: categories}, nil
}

// addMonthsClamped shifts t by months, clamping the day to the target month's
// last day (matching chrono's Months arithmetic).
func addMonthsClamped(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	target := time.Date(y, m, 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location()).AddDate(0, months, 0)
	lastDay := time.Date(target.Year(), target.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if d > lastDay {
		d = lastDay
	}
	return time.Date(target.Year(), target.Month(), d, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

// bucketLabel renders a bucket key like "5 Nov" (day) or "Nov 26" (month).
func bucketLabel(key, gran string) string {
	if gran == "month" {
		if d, err := time.Parse("2006-01", key); err == nil {
			return d.Format("Jan 06")
		}
		return key
	}
	if d, err := time.Parse("2006-01-02", key); err == nil {
		return strconv.Itoa(d.Day()) + " " + d.Format("Jan")
	}
	return key
}
