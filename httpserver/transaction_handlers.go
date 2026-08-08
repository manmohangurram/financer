package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mohan9182/financer/api"
)

// --- transactions ---

type reqTxn struct {
	Transactions jsonListTxn `json:"transactions"`
	PageSize     int32       `json:"pageSize"`
	PageToken    string      `json:"pageToken"`
	AccountId    string      `json:"accountId"`
	CategoryId   []string    `json:"categoryId"`
	Names        []string    `json:"names"`
	Type         any         `json:"type"`
	Ids          []string    `json:"ids"`
}

type jsonListTxn []struct {
	Id          string   `json:"id"`
	Name        string   `json:"name"`
	Amount      float64  `json:"amount"`
	Type        any      `json:"type"`
	OccurredAt  any      `json:"occurredAt"`
	AccountId   string   `json:"accountId"`
	CategoryIds []string `json:"categoryIds"`
	ExternalId  string   `json:"externalId"`
}

type wireTxn struct {
	Id               string   `json:"id"`
	Name             string   `json:"name"`
	Amount           float64  `json:"amount"`
	Type             string   `json:"type"`
	OccurredAt       string   `json:"occurredAt"`
	AccountId        string   `json:"accountId"`
	CreatedAt        string   `json:"createdAt"`
	LinkedTransferId string   `json:"linkedTransferId"`
	CategoryIds      []string `json:"categoryIds"`
}

func txnWire(t *api.TransactionResponse) wireTxn {
	return wireTxn{
		Id:               t.Id,
		Name:             t.Name,
		Amount:           cents(t.Amount),
		Type:             t.Type.String(),
		OccurredAt:       tsRFC3339(t.OccurredAt),
		AccountId:        t.AccountId,
		CreatedAt:        tsRFC3339(t.CreatedAt),
		LinkedTransferId: t.LinkedTransferId,
		CategoryIds:      t.CategoryIds,
	}
}

func txnTypeValue(v any) (api.TransactionType, error) {
	switch x := v.(type) {
	case nil:
		return 0, nil
	case string:
		if n, ok := api.TransactionType_value[x]; ok {
			return api.TransactionType(n), nil
		}
		return 0, fmt.Errorf("unknown transaction type %q", x)
	case float64:
		return api.TransactionType(x), nil
	default:
		return 0, fmt.Errorf("invalid transaction type %v", v)
	}
}

// txnOccurredAt accepts {seconds,nanos} or an RFC3339/date string.
func txnOccurredAt(v any) (time.Time, error) {
	switch x := v.(type) {
	case nil:
		return time.Time{}, nil
	case map[string]any:
		sec, _ := x["seconds"].(float64)
		nanos, _ := x["nanos"].(float64)
		return time.Unix(int64(sec), int64(nanos)).UTC(), nil
	case string:
		t, err := time.Parse(time.RFC3339, x)
		if err != nil {
			t, err = time.Parse("2006-01-02", x)
		}
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid date %q", x)
		}
		return t.UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("invalid date %v", v)
	}
}

func (a *API) listTransactions(ctx context.Context, _ string, r *http.Request) (any, error) {
	q := r.URL.Query()
	in := reqTxn{PageSize: queryInt(q, "pageSize"), PageToken: q.Get("pageToken"), AccountId: q.Get("accountId")}
	if v := q.Get("type"); v != "" {
		in.Type = v
	}
	if v := q.Get("categoryId"); v != "" {
		in.CategoryId = strings.Split(v, ",")
	}
	if v := q.Get("names"); v != "" {
		in.Names = strings.Split(v, ",")
	}
	t, err := txnTypeValue(in.Type)
	if err != nil {
		return nil, bad("%v", err)
	}
	dateFrom, _ := queryDate(q, "dateFrom")
	dateTo, _ := queryDate(q, "dateTo")
	minAmount := queryFloat(q, "minAmount")
	maxAmount := queryFloat(q, "maxAmount")
	out, err := a.txn.ListTransactions(ctx, &api.ListTransactionsRequest{
		PageSize: in.PageSize, PageToken: in.PageToken, AccountId: in.AccountId, CategoryId: in.CategoryId, Type: t,
		DateFrom: dateFrom, DateTo: dateTo, MinAmount: minAmount, MaxAmount: maxAmount,
		Names: in.Names,
		SortBy: q.Get("sortBy"), SortDir: q.Get("sortDir"), Offset: queryInt(q, "offset"),
	})
	if err != nil {
		return nil, err
	}
	items := make([]wireTxn, 0, len(out.Transactions))
	for _, t := range out.Transactions {
		items = append(items, txnWire(t))
	}
	return struct {
		Transactions  []wireTxn `json:"transactions"`
		NextPageToken string    `json:"nextPageToken"`
		TotalCount    int32     `json:"totalCount"`
	}{Transactions: items, NextPageToken: out.NextPageToken, TotalCount: out.TotalCount}, nil
}

func (a *API) createTransactions(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqTxn
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	var txns []*api.CreateTransactionRequest
	for _, t := range in.Transactions {
		ty, err := txnTypeValue(t.Type)
		if err != nil {
			return nil, bad("%v", err)
		}
		occ, err := txnOccurredAt(t.OccurredAt)
		if err != nil {
			return nil, bad("%v", err)
		}
		txns = append(txns, &api.CreateTransactionRequest{
			Name: t.Name, Amount: float32(t.Amount), Type: ty, OccurredAt: occ, AccountId: t.AccountId, CategoryIds: t.CategoryIds, ExternalId: t.ExternalId,
		})
	}
	out, err := a.txn.CreateTransactions(ctx, &api.CreateTransactionsRequest{Transactions: txns})
	if err != nil {
		return nil, err
	}
	return created(bulkWire(out)), nil
}

func (a *API) updateTransactions(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqTxn
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	var txns []*api.UpdateTransactionRequest
	for _, t := range in.Transactions {
		ty, err := txnTypeValue(t.Type)
		if err != nil {
			return nil, bad("%v", err)
		}
		occ, err := txnOccurredAt(t.OccurredAt)
		if err != nil {
			return nil, bad("%v", err)
		}
		txns = append(txns, &api.UpdateTransactionRequest{
			Id: t.Id, Name: t.Name, Amount: float32(t.Amount), Type: ty, OccurredAt: occ, AccountId: t.AccountId, CategoryIds: t.CategoryIds,
		})
	}
	out, err := a.txn.UpdateTransactions(ctx, &api.UpdateTransactionsRequest{Transactions: txns})
	if err != nil {
		return nil, err
	}
	return bulkWire(out), nil
}

func (a *API) deleteTransactions(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqTxn
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	_, err := a.txn.DeleteTransactions(ctx, &api.DeleteTransactionsRequest{Ids: in.Ids})
	if err != nil {
		return nil, err
	}
	return noContent(), nil
}
