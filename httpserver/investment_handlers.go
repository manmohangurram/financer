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
	InvestmentId   string  `json:"investmentId"`
	Symbol         string  `json:"symbol"`
	Name           string  `json:"name"`
	InvestmentType any     `json:"investmentType"`
	ManualNav      float64 `json:"manualNav"`
	Side           any     `json:"side"`
	Quantity       float64 `json:"quantity"`
	Price          float64 `json:"price"`
	OccurredAt     any     `json:"occurredAt"`
	Query          string  `json:"query"`
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
	return investmentWire(out), nil
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
		Id: in.Id, Name: in.Name, InvestmentType: t, ManualNav: float32(in.ManualNav),
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
	out, err := a.invest.DeleteInvestment(ctx, &api.DeleteInvestmentRequest{Id: in.Id})
	if err != nil {
		return nil, err
	}
	return wireOp{Success: out.Success, Message: out.Message}, nil
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
	return lotWire(out), nil
}

func (a *API) deleteLot(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqInvestment
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	in.Id = r.PathValue("id")
	out, err := a.invest.DeleteLot(ctx, &api.DeleteLotRequest{Id: in.Id})
	if err != nil {
		return nil, err
	}
	return wireOp{Success: out.Success, Message: out.Message}, nil
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
	points, err := a.invest.GetPriceHistory(ctx, r.PathValue("id"), r.URL.Query().Get("range"), r.URL.Query().Get("refresh") == "1")
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
