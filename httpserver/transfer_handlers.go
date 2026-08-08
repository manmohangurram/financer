package httpserver

import (
	"context"
	"net/http"

	"github.com/mohan9182/financer/api"
)

// --- transfers ---

type reqTransfer struct {
	Ids   []string `json:"ids"`
	Links []struct {
		DebitTransactionId  string `json:"debitTransactionId"`
		CreditTransactionId string `json:"creditTransactionId"`
	} `json:"links"`
}

type reqCreateCounterpart struct {
	TransactionId string `json:"transactionId"`
	ToAccountId   string `json:"toAccountId"`
}

func (a *API) linkTransfers(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqTransfer
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	var links []*api.LinkTransferRequest
	for _, l := range in.Links {
		links = append(links, &api.LinkTransferRequest{
			DebitTransactionId: l.DebitTransactionId, CreditTransactionId: l.CreditTransactionId,
		})
	}
	out, err := a.transfer.LinkTransfers(ctx, &api.LinkTransfersRequest{Links: links})
	if err != nil {
		return nil, err
	}
	return created(bulkWire(out)), nil
}

func (a *API) createCounterpart(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqCreateCounterpart
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	out, err := a.transfer.CreateCounterpart(ctx, &api.CreateCounterpartRequest{TransactionId: in.TransactionId, ToAccountId: in.ToAccountId})
	if err != nil {
		return nil, err
	}
	return created(out), nil
}

func (a *API) unlinkTransfers(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqTransfer
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	_, err := a.transfer.UnlinkTransfers(ctx, &api.UnlinkTransfersRequest{Ids: in.Ids})
	if err != nil {
		return nil, err
	}
	return noContent(), nil
}
