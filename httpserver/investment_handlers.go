package httpserver

import (
	"context"
	"net/http"

	"github.com/mohan9182/financer/api"
	"github.com/mohan9182/financer/services"
)

// --- investments ---

type reqInvestment struct {
	Id             string  `json:"id"`
	Symbol         string  `json:"symbol"`
	Name           string  `json:"name"`
	InvestmentType any     `json:"investmentType"`
	ManualNav      float64 `json:"manualNav"`
	Side           any     `json:"side"`
	Quantity       float64 `json:"quantity"`
	Price          float64 `json:"price"`
	OccurredAt     any     `json:"occurredAt"`
}

func (a *API) createInvestment(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqInvestment
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	t, err := investmentTypeValue(in.InvestmentType)
	if err != nil {
		return nil, bad("%v", err)
	}
	out, err := a.invest.CreateInvestment(ctx, &api.CreateInvestmentRequest{
		Symbol: in.Symbol, Name: in.Name, InvestmentType: t, ManualNav: float32(in.ManualNav),
	})
	if err != nil {
		return nil, err
	}
	return created(investmentWire(out)), nil
}

func (a *API) listInvestments(ctx context.Context, _ string, _ *http.Request) (any, error) {
	out, err := a.invest.ListInvestments(ctx, &api.ListInvestmentsRequest{})
	if err != nil {
		return nil, err
	}
	items := make([]wireInvestment, 0, len(out.Investments))
	for _, i := range out.Investments {
		items = append(items, investmentWire(i))
	}
	return struct {
		Investments []wireInvestment `json:"investments"`
	}{Investments: items}, nil
}

func (a *API) getInvestment(ctx context.Context, _ string, r *http.Request) (any, error) {
	out, err := a.invest.GetInvestment(ctx, &api.GetInvestmentRequest{Id: r.PathValue("id")})
	if err != nil {
		return nil, err
	}
	return investmentWire(out), nil
}

func (a *API) updateInvestment(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqInvestment
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	t, err := investmentTypeValue(in.InvestmentType)
	if err != nil {
		return nil, bad("%v", err)
	}
	in.Id = r.PathValue("id")
	out, err := a.invest.UpdateInvestment(ctx, &api.UpdateInvestmentRequest{
		Id: in.Id, Symbol: in.Symbol, Name: in.Name, InvestmentType: t, ManualNav: float32(in.ManualNav),
	})
	if err != nil {
		return nil, err
	}
	return investmentWire(out), nil
}

func (a *API) deleteInvestment(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqInvestment
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	in.Id = r.PathValue("id")
	_, err := a.invest.DeleteInvestment(ctx, &api.DeleteInvestmentRequest{Id: in.Id})
	if err != nil {
		return nil, err
	}
	return noContent(), nil
}

func (a *API) addLot(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqInvestment
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	side, err := lotSideValue(in.Side)
	if err != nil {
		return nil, bad("%v", err)
	}
	occ, err := txnOccurredAt(in.OccurredAt)
	if err != nil {
		return nil, bad("%v", err)
	}
	out, err := a.invest.AddLot(ctx, &api.AddLotRequest{
		InvestmentId: r.PathValue("id"), Side: side, Quantity: float32(in.Quantity), Price: float32(in.Price), OccurredAt: occ,
	})
	if err != nil {
		return nil, err
	}
	return created(lotWire(out)), nil
}

func (a *API) deleteLot(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqInvestment
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	_, err := a.invest.DeleteLot(ctx, &api.DeleteLotRequest{Id: r.PathValue("lotId")})
	if err != nil {
		return nil, err
	}
	return noContent(), nil
}

func (a *API) updateLot(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqInvestment
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	occ, err := txnOccurredAt(in.OccurredAt)
	if err != nil {
		return nil, bad("%v", err)
	}
	out, err := a.invest.UpdateLot(ctx, &api.UpdateLotRequest{
		Id: r.PathValue("lotId"), InvestmentId: r.PathValue("id"),
		Quantity: float32(in.Quantity), Price: float32(in.Price), OccurredAt: occ,
	})
	if err != nil {
		return nil, err
	}
	return lotWire(out), nil
}

func (a *API) listLots(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqInvestment
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	lots, err := a.invest.ListLots(ctx, &api.GetInvestmentRequest{Id: r.PathValue("id")})
	if err != nil {
		return nil, err
	}
	items := make([]wireLot, 0, len(lots))
	for _, l := range lots {
		items = append(items, lotWire(l))
	}
	return struct {
		Lots []wireLot `json:"lots"`
	}{Lots: items}, nil
}

func (a *API) priceHistory(ctx context.Context, _ string, r *http.Request) (any, error) {
	q := r.URL.Query()
	points, err := a.invest.GetPriceHistory(ctx, r.PathValue("id"), q.Get("range"), q.Get("from"), q.Get("to"), q.Get("refresh") == "1")
	if err != nil {
		return nil, err
	}
	return struct {
		Points []services.PricePoint `json:"points"`
	}{Points: points}, nil
}

func (a *API) searchSymbols(ctx context.Context, _ string, r *http.Request) (any, error) {
	out, err := a.invest.SearchSymbols(ctx, &api.SearchSymbolsRequest{Query: r.URL.Query().Get("query")})
	if err != nil {
		return nil, err
	}
	results := make([]struct {
		Symbol         string `json:"symbol"`
		Name           string `json:"name"`
		InvestmentType string `json:"investmentType"`
	}, 0, len(out.Results))
	for _, res := range out.Results {
		results = append(results, struct {
			Symbol         string `json:"symbol"`
			Name           string `json:"name"`
			InvestmentType string `json:"investmentType"`
		}{Symbol: res.Symbol, Name: res.Name, InvestmentType: res.InvestmentType.String()})
	}
	return struct {
		Results []struct {
			Symbol         string `json:"symbol"`
			Name           string `json:"name"`
			InvestmentType string `json:"investmentType"`
		} `json:"results"`
	}{Results: results}, nil
}

func (a *API) refreshPrices(ctx context.Context, _ string, _ *http.Request) (any, error) {
	out, err := a.invest.RefreshPrices(ctx, &api.RefreshPricesRequest{})
	if err != nil {
		return nil, err
	}
	return struct {
		Updated int32 `json:"updated"`
	}{Updated: out.Updated}, nil
}

func (a *API) getPortfolioSummary(ctx context.Context, _ string, _ *http.Request) (any, error) {
	out, err := a.invest.GetPortfolioSummary(ctx, &api.GetPortfolioSummaryRequest{})
	if err != nil {
		return nil, err
	}
	return wirePortfolioSummary{
		TotalInvested:      cents(out.TotalInvested),
		TotalCurrentValue:  cents(out.TotalCurrentValue),
		TotalUnrealizedPnl: cents(out.TotalUnrealizedPnl),
		TotalRealizedPnl:   cents(out.TotalRealizedPnl),
	}, nil
}
