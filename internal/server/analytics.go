package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mohan9182/financer/internal/model"
)

// dashboard returns the headline totals plus the account list in one call.
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
	c.JSON(http.StatusOK, model.WireDashboard{
		TotalBalance:  round2(dash.TotalBalance),
		TotalIncome:   round2(dash.TotalIncome),
		TotalExpenses: round2(dash.TotalExpenses),
		Accounts:      accounts,
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
