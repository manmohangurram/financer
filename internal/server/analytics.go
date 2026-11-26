package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
	tim "github.com/mohan9182/financer/internal/time"
)

// dashboard returns the account position plus the current month at a glance:
// month totals, budget progress, daily spend and the latest entries. All money
// math happens here; the frontend only formats.
func (a *API) dashboard(c *gin.Context) {
	uid := c.GetString(userIDKey)
	dash, err := a.txnSvc.Dashboard(uid)
	if err != nil {
		writeError(c, err)
		return
	}
	accounts, err := a.accountSvc.List(uid)
	if err != nil {
		writeError(c, err)
		return
	}

	now := time.Now().In(tim.Loc)
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, tim.Loc)
	to := from.AddDate(0, 1, -1)
	fromStr := from.Format("2006-01-02")
	toStr := to.Format("2006-01-02")

	month, err := a.txnSvc.MonthTotals(uid, fromStr, toStr)
	if err != nil {
		writeError(c, err)
		return
	}
	prevFrom := from.AddDate(0, -1, 0)
	prevTo := from.AddDate(0, 0, -1)
	previous, err := a.txnSvc.MonthTotals(uid, prevFrom.Format("2006-01-02"), prevTo.Format("2006-01-02"))
	if err != nil {
		writeError(c, err)
		return
	}
	spending, err := a.txnSvc.Spending(uid, "", fromStr, toStr, "")
	if err != nil {
		writeError(c, err)
		return
	}
	categories, _, err := a.categorySvc.List(uid, 10000, "")
	if err != nil {
		writeError(c, err)
		return
	}
	recent, err := a.txnSvc.List(uid, &repo.TransactionListFilter{PageSize: 6})
	if err != nil {
		writeError(c, err)
		return
	}

	cash, savings, debt := 0.0, 0.0, 0.0
	for _, acc := range accounts {
		switch acc.AccountType {
		case model.AccountTypeCurrent:
			cash += acc.Balance
		case model.AccountTypeSavings:
			savings += acc.Balance
		case model.AccountTypeLoan, model.AccountTypeCreditCard:
			debt -= acc.Balance
		}
	}

	daysElapsed := now.Day()
	if daysElapsed < 1 {
		daysElapsed = 1
	}
	daysLeft := to.Day() - now.Day()
	if daysLeft < 0 {
		daysLeft = 0
	}
	avgPerDay := 0.0
	if daysElapsed > 0 {
		avgPerDay = month.Expenses / float64(daysElapsed)
	}
	changePct := 0.0
	if previous.Expenses > 0 {
		changePct = (month.Expenses - previous.Expenses) / previous.Expenses * 100
	}

	// Daily buckets for the chart, with zero days present so the line never
	// skips a day. The chart stops at today, not the end of the month.
	byKey := make(map[string]model.WireSpendingBucket, len(spending.Buckets))
	for _, b := range spending.Buckets {
		byKey[b.Key] = model.WireSpendingBucket{Key: b.Key, Label: b.Label, Amount: round2(b.Amount)}
	}
	buckets := make([]model.WireSpendingBucket, 0, daysElapsed)
	var peak *model.WireSpendingBucket
	for d := 1; d <= daysElapsed; d++ {
		day := time.Date(now.Year(), now.Month(), d, 0, 0, 0, 0, tim.Loc)
		key := day.Format("2006-01-02")
		b, ok := byKey[key]
		if !ok {
			b = model.WireSpendingBucket{Key: key, Label: day.Format("2 Jan")}
		}
		buckets = append(buckets, b)
		if b.Amount > 0 && (peak == nil || b.Amount > peak.Amount) {
			copied := b
			peak = &copied
		}
	}

	// The largest category is the one the spending card names; it is not the
	// same as the over-budget category.
	var top *model.WireDashboardTopCategory
	for _, cat := range spending.Categories {
		if cat.Debit <= 0 {
			continue
		}
		if top == nil || cat.Debit > top.Amount {
			share := 0.0
			if month.Expenses > 0 {
				share = cat.Debit / month.Expenses * 100
			}
			top = &model.WireDashboardTopCategory{
				Name:   cat.Name,
				Amount: round2(cat.Debit),
				Share:  round2(share),
			}
		}
	}

	// Budgets are monthly and global: sum the limits and the spend in those
	// categories only. Unbudgeted spend still belongs in the spending card.
	spendByCategory := make(map[string]float64, len(spending.Categories))
	for _, cat := range spending.Categories {
		spendByCategory[cat.ID] = cat.Debit
	}
	planned, budgetSpent := 0.0, 0.0
	overCount := 0
	for _, cat := range categories {
		if cat.MonthlyBudget <= 0 {
			continue
		}
		planned += cat.MonthlyBudget
		spent := spendByCategory[cat.ID]
		budgetSpent += spent
		if spent > cat.MonthlyBudget {
			overCount++
		}
	}
	used := 0.0
	if planned > 0 {
		used = budgetSpent / planned * 100
	}

	recentItems := wireTransactions(recent.Rows)
	recentTotal := 0.0
	for _, row := range recent.Rows {
		if row.Txn.Type == string(model.TransactionTypeDebit) {
			recentTotal -= row.Txn.Amount
		} else {
			recentTotal += row.Txn.Amount
		}
	}

	c.JSON(http.StatusOK, model.WireDashboard{
		TotalBalance:  round2(dash.TotalBalance),
		TotalIncome:   round2(dash.TotalIncome),
		TotalExpenses: round2(dash.TotalExpenses),
		Cash:          round2(cash),
		Savings:       round2(savings),
		Debt:          round2(debt),
		Accounts:      accounts,
		Month: model.WireDashboardMonth{
			Label:            now.Format("January 2006"),
			From:             fromStr,
			To:               toStr,
			DaysLeft:         daysLeft,
			Income:           round2(month.Income),
			Expenses:         round2(month.Expenses),
			Net:              round2(month.Income - month.Expenses),
			AveragePerDay:    round2(avgPerDay),
			PreviousLabel:    prevFrom.Format("January 2006"),
			PreviousExpenses: round2(previous.Expenses),
			ExpenseChangePct: round2(changePct),
			TopCategory:      top,
			PeakBucket:       peak,
			Budget: model.WireDashboardBudget{
				Planned:    round2(planned),
				Spent:      round2(budgetSpent),
				Left:       round2(planned - budgetSpent),
				Used:       round2(used),
				HasBudgets: planned > 0,
				OverCount:  overCount,
			},
			Buckets: buckets,
		},
		Recent:      recentItems,
		RecentTotal: round2(recentTotal),
	})
}

// spending returns day/month buckets and per-category totals for a range.
func (a *API) spending(c *gin.Context) {
	uid := c.GetString(userIDKey)
	q := c.Request.URL.Query()
	res, err := a.txnSvc.Spending(uid, q.Get("range"), q.Get("from"), q.Get("to"), q.Get("accountId"))
	if err != nil {
		writeError(c, err)
		return
	}
	buckets := make([]model.WireSpendingBucket, 0, len(res.Buckets))
	for _, b := range res.Buckets {
		buckets = append(buckets, model.WireSpendingBucket{
			Key:    b.Key,
			Label:  b.Label,
			Amount: round2(b.Amount),
		})
	}
	categories := make([]model.WireSpendingCategory, 0, len(res.Categories))
	for _, cat := range res.Categories {
		categories = append(categories, model.WireSpendingCategory{
			ID:     cat.ID,
			Name:   cat.Name,
			Debit:  round2(cat.Debit),
			Credit: round2(cat.Credit),
			Net:    round2(cat.Net),
		})
	}
	c.JSON(http.StatusOK, model.WireSpending{Buckets: buckets, Categories: categories})
}
