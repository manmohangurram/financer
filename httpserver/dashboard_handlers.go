package httpserver

import (
	"context"
	"net/http"

	"github.com/mohan9182/financer/api"
)

// dashboard is a single request that assembles the whole summary server-side
// (no client-side math): balance/income/expense sums + portfolio + accounts +
// investments.
func (a *API) dashboard(ctx context.Context, _ string, _ *http.Request) (any, error) {
	d, err := a.txn.Dashboard(ctx)
	if err != nil {
		return nil, err
	}
	summary, err := a.invest.GetPortfolioSummary(ctx, &api.GetPortfolioSummaryRequest{})
	if err != nil {
		return nil, err
	}
	accountsResp, err := a.account.ListAccounts(ctx, &api.ListAccountsRequest{})
	if err != nil {
		return nil, err
	}
	instResp, err := a.invest.ListInvestments(ctx, &api.ListInvestmentsRequest{})
	if err != nil {
		return nil, err
	}

	accounts := make([]wireAccount, 0, len(accountsResp.Accounts))
	for _, acc := range accountsResp.Accounts {
		accounts = append(accounts, accountWire(acc))
	}
	investments := make([]wireInvestment, 0, len(instResp.Investments))
	for _, inst := range instResp.Investments {
		investments = append(investments, investmentWire(inst))
	}

	return struct {
		TotalBalance   float64          `json:"totalBalance"`
		TotalIncome    float64          `json:"totalIncome"`
		TotalExpenses  float64          `json:"totalExpenses"`
		PortfolioValue float64          `json:"portfolioValue"`
		Accounts       []wireAccount    `json:"accounts"`
		Investments    []wireInvestment `json:"investments"`
	}{
		TotalBalance:   cents(d.TotalBalance),
		TotalIncome:    cents(d.TotalIncome),
		TotalExpenses:  cents(d.TotalExpenses),
		PortfolioValue: cents(summary.TotalCurrentValue),
		Accounts:       accounts,
		Investments:    investments,
	}, nil
}
