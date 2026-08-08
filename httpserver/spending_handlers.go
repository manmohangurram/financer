package httpserver

import (
	"context"
	"net/http"

	"github.com/mohan9182/financer/api"
)

// spending returns day/month buckets and per-category debit/credit/net for a
// range (7D/1M/6M/1Y or custom from/to), computed server-side in SQL.
func (a *API) spending(ctx context.Context, _ string, r *http.Request) (any, error) {
	q := r.URL.Query()
	out, err := a.txn.Spending(ctx, &api.SpendingRequest{
		Range: q.Get("range"), From: q.Get("from"), To: q.Get("to"), AccountId: q.Get("accountId"),
	})
	if err != nil {
		return nil, err
	}
	buckets := make([]wireSpendingBucket, 0, len(out.Buckets))
	for _, b := range out.Buckets {
		buckets = append(buckets, wireSpendingBucket{Key: b.Key, Label: b.Label, Amount: b.Amount})
	}
	cats := make([]wireSpendingCategory, 0, len(out.Categories))
	for _, c := range out.Categories {
		cats = append(cats, wireSpendingCategory{Id: c.Id, Name: c.Name, Debit: c.Debit, Credit: c.Credit, Net: c.Net})
	}
	return struct {
		Buckets    []wireSpendingBucket   `json:"buckets"`
		Categories []wireSpendingCategory `json:"categories"`
	}{Buckets: buckets, Categories: cats}, nil
}

type wireSpendingBucket struct {
	Key    string  `json:"key"`
	Label  string  `json:"label"`
	Amount float32 `json:"amount"`
}

type wireSpendingCategory struct {
	Id     string  `json:"id"`
	Name   string  `json:"name"`
	Debit  float32 `json:"debit"`
	Credit float32 `json:"credit"`
	Net    float32 `json:"net"`
}
