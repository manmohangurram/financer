package server

import (
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
	tim "github.com/mohan9182/financer/internal/time"
	"github.com/mohan9182/financer/internal/transactionservice"
)

// txnOccurredAt converts the flexible occurredAt input ({seconds,nanos},
// RFC3339, or YYYY-MM-DD) into a stored UTC timestamp.
func txnOccurredAt(raw json.RawMessage) (string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", nil
	}
	if strings.HasPrefix(trimmed, "{") {
		var obj struct {
			Seconds int64 `json:"seconds"`
			Nanos   int64 `json:"nanos"`
		}
		if err := json.Unmarshal(raw, &obj); err != nil {
			return "", errors.BadRequest("invalid date")
		}
		t := time.Unix(obj.Seconds, obj.Nanos).UTC()
		return tim.FromUTC(t), nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", errors.BadRequest("invalid date")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return tim.FromUTC(t.UTC()), nil
	}
	if d, err := time.Parse("2006-01-02", s); err == nil {
		return tim.FromUTC(d.UTC()), nil
	}
	return "", errors.BadRequest("invalid date " + strconv.Quote(s))
}

func toRequests(in []model.ReqListTxn, forUpdate bool) ([]transactionservice.Request, error) {
	reqs := make([]transactionservice.Request, 0, len(in))
	for _, t := range in {
		if !validTxnType(t.Type) {
			return nil, errors.BadRequest("unknown transaction type " + strconv.Quote(string(t.Type)))
		}
		occ, err := txnOccurredAt(t.OccurredAt)
		if err != nil {
			return nil, err
		}
		id := t.ID
		if !forUpdate && id == "" {
			id = newUUID()
		}
		var externalID *string
		if t.ExternalID != "" {
			externalID = &t.ExternalID
		}
		reqs = append(reqs, transactionservice.Request{
			ID:          id,
			Name:        t.Name,
			Amount:      t.Amount,
			Type:        string(t.Type),
			OccurredAt:  occ,
			AccountID:   t.AccountID,
			CategoryIDs: t.CategoryIDs,
			ExternalID:  externalID,
		})
	}
	return reqs, nil
}

func validTxnType(t model.TransactionType) bool {
	return t == model.TransactionTypeDebit || t == model.TransactionTypeCredit
}

func (a *API) listTransactions(c *gin.Context) {
	uid := c.GetString(userIDKey)
	q := c.Request.URL.Query()

	var typ *string
	if s := q.Get("type"); s != "" {
		if !validTxnType(model.TransactionType(s)) {
			writeError(c, errors.BadRequest("unknown transaction type "+strconv.Quote(s)))
			return
		}
		t := s
		typ = &t
	}

	f := &repo.TransactionListFilter{
		AccountID:   q.Get("accountId"),
		CategoryIDs: splitCSV(q.Get("categoryId")),
		Type:        typ,
		DateFrom:    q.Get("dateFrom"),
		DateTo:      q.Get("dateTo"),
		MinAmount:   parseFloat(q.Get("minAmount")),
		MaxAmount:   parseFloat(q.Get("maxAmount")),
		Names:       splitCSV(q.Get("names")),
		PageSize:    int64(parseInt(q.Get("pageSize"), 0)),
		PageToken:   q.Get("pageToken"),
		SortBy:      q.Get("sortBy"),
		SortDir:     q.Get("sortDir"),
		Offset:      int64(parseInt(q.Get("offset"), 0)),
	}
	res, err := a.txnSvc.List(uid, f)
	if err != nil {
		writeError(c, err)
		return
	}
	items := make([]model.WireTxn, 0, len(res.Rows))
	for _, row := range res.Rows {
		name := row.Txn.Name
		if row.Txn.CleanName != nil {
			name = *row.Txn.CleanName
		}
		var catIDs *[]string
		if len(row.CategoryIDs) > 0 {
			ids := row.CategoryIDs
			catIDs = &ids
		}
		items = append(items, model.WireTxn{
			ID:               row.Txn.ID,
			Name:             name,
			Amount:           round2(row.Txn.Amount),
			Type:             row.Txn.Type,
			OccurredAt:       tim.TSRFC3339(row.Txn.OccurredAt),
			AccountID:        row.Txn.AccountID,
			CreatedAt:        tim.TSRFC3339(row.Txn.CreatedAt),
			LinkedTransferID: row.LinkID,
			CategoryIDs:      catIDs,
		})
	}
	c.JSON(http.StatusOK, model.WireTxnList{
		Transactions:  items,
		NextPageToken: res.NextPageToken,
		TotalCount:    res.TotalCount,
	})
}

func (a *API) createTransactions(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var in model.ReqTxn
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	reqs, err := toRequests(in.Transactions, false)
	if err != nil {
		writeError(c, err)
		return
	}
	out, err := a.txnSvc.Create(uid, reqs)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (a *API) updateTransactions(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var in model.ReqTxn
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	reqs, err := toRequests(in.Transactions, true)
	if err != nil {
		writeError(c, err)
		return
	}
	out, err := a.txnSvc.Update(uid, reqs)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (a *API) deleteTransactions(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var in model.ReqTxn
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	if _, err := a.txnSvc.Delete(uid, in.IDs); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// --- transfer links ---

func (a *API) linkTransfers(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var in model.ReqTransfer
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	links := make([][2]string, 0, len(in.Links))
	for _, l := range in.Links {
		links = append(links, [2]string{l.DebitTransactionID, l.CreditTransactionID})
	}
	out, err := a.transferSvc.LinkTransfers(uid, links)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (a *API) unlinkTransfers(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var in model.ReqTransfer
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	if _, err := a.transferSvc.UnlinkTransfers(uid, in.IDs); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *API) createTransferCounterpart(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var in model.ReqCreateCounterpart
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	out, err := a.transferSvc.CreateCounterpart(uid, in.TransactionID, in.ToAccountID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

// --- rules ---

func (a *API) listRules(c *gin.Context) {
	uid := c.GetString(userIDKey)
	pageSize := int64(parseInt(c.Query("pageSize"), 0))
	pageToken := c.Query("pageToken")
	if pageSize > 0 || pageToken != "" {
		rules, next, err := a.ruleSvc.ListPage(uid, pageSize, pageToken)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, model.WireRuleList{Rules: rules, NextPageToken: next})
		return
	}
	rules, err := a.ruleSvc.List(uid)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, model.WireRuleList{Rules: rules})
}

func (a *API) createRule(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var in model.ReqRule
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	out, err := a.ruleSvc.Create(uid, in.Name, in.Priority, string(in.Logic), in.Conditions, in.Actions)
	if err != nil {
		writeError(c, err)
		return
	}
	// Backfill existing transactions; failures are non-fatal.
	if _, err := a.ruleSvc.ApplyToExisting(uid); err != nil {
		slog.Warn("rule backfill failed", "rule", out.ID, "err", err)
	}
	c.JSON(http.StatusCreated, out)
}

func (a *API) updateRule(c *gin.Context) {
	uid := c.GetString(userIDKey)
	id := c.Param("id")
	var in model.ReqRule
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	out, err := a.ruleSvc.Update(uid, id, in.Name, in.Priority, string(in.Logic), in.Conditions, in.Actions)
	if err != nil {
		writeError(c, err)
		return
	}
	if _, err := a.ruleSvc.ApplyToExisting(uid); err != nil {
		slog.Warn("rule backfill failed", "rule", id, "err", err)
	}
	c.JSON(http.StatusOK, out)
}

func (a *API) deleteRule(c *gin.Context) {
	uid := c.GetString(userIDKey)
	if err := a.ruleSvc.Delete(uid, c.Param("id")); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *API) previewRules(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var in model.ReqPreview
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	views, err := a.ruleSvc.Preview(uid, string(in.Logic), in.Conditions, in.Limit)
	if err != nil {
		writeError(c, err)
		return
	}
	items := make([]model.WireTxnPreview, 0, len(views))
	for _, v := range views {
		items = append(items, model.WireTxnPreview{
			Name:        v.Name,
			Amount:      v.Amount,
			Type:        v.Type,
			AccountID:   v.AccountID,
			CategoryIDs: v.CategoryIDs,
			OccurredAt:  tim.TSRFC3339(v.OccurredAt),
		})
	}
	c.JSON(http.StatusOK, model.WirePreview{Transactions: items})
}

func (a *API) runRule(c *gin.Context) {
	uid := c.GetString(userIDKey)
	id := c.Param("id")
	updated, err := a.ruleSvc.Run(uid, id)
	if err != nil {
		writeError(c, err)
		return
	}
	tr, err := a.transferRule.RunRule(uid, id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, model.WireRuleRun{
		Updated: updated,
		Matched: tr.Matched,
		Linked:  tr.Linked,
		Created: tr.Created,
	})
}

// --- small helpers ---

func newUUID() string {
	return uuid.New().String()
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
