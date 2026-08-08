package services

import (
	"context"
	"fmt"
	"github.com/mohan9182/financer/logx"
	"time"

	"github.com/mohan9182/financer/auth"
	"github.com/mohan9182/financer/api"
	"github.com/mohan9182/financer/repository"
)

type InvestmentService struct {
	repo  *repository.InvestmentRepository
	yahoo *YahooClient
}

func NewInvestmentService(repo *repository.InvestmentRepository, yahoo *YahooClient) *InvestmentService {
	return &InvestmentService{repo: repo, yahoo: yahoo}
}

// effectivePrice returns manual_nav if set, else the cached Yahoo price.
func effectivePrice(inst *api.InvestmentResponse) float32 {
	if inst.ManualNav > 0 {
		return inst.ManualNav
	}
	return inst.CurrentPrice
}

// positionFor computes the FIFO position, its current value, and unrealized P&L
// for one investment. currentValue = remaining quantity * effective price;
// unrealized = (price - avgCost) * quantity.
func (s *InvestmentService) positionFor(ctx context.Context, inst *api.InvestmentResponse) (Position, float32, float32, error) {
	lots, err := s.repo.ListLots(ctx, inst.Id)
	if err != nil {
		return Position{}, 0, 0, err
	}
	fifoLots := make([]FIFOLot, 0, len(lots))
	for _, l := range lots {
		fifoLots = append(fifoLots, FIFOLot{Side: l.Side, Quantity: l.Quantity, Price: l.Price})
	}
	pos := ComputeFIFO(fifoLots)
	price := effectivePrice(inst)
	currentValue := pos.Quantity * price
	unrealized := (price - pos.AvgCost) * pos.Quantity
	return pos, currentValue, unrealized, nil
}

func (s *InvestmentService) CreateInvestment(ctx context.Context, msg *api.CreateInvestmentRequest) (*api.InvestmentResponse, error) {
	if msg.Name == "" {
		return nil, BadRequest("name is required")
	}
	if msg.InvestmentType != api.InvestmentType_INVESTMENT_TYPE_STOCK &&
		msg.InvestmentType != api.InvestmentType_INVESTMENT_TYPE_MUTUAL_FUND {
		return nil, BadRequest("INVESTMENT_TYPE must be STOCK or MUTUAL_FUND")
	}
	if msg.InvestmentType == api.InvestmentType_INVESTMENT_TYPE_STOCK && msg.Symbol == "" {
		return nil, BadRequest("symbol is required for STOCK")
	}

	userID := ctx.Value(auth.UserIDKey).(string)
	inst, err := s.repo.CreateInvestment(ctx, userID, msg.Symbol, msg.Name, msg.InvestmentType, msg.ManualNav)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	return s.withPosition(ctx, inst)
}

func (s *InvestmentService) GetInvestment(ctx context.Context, msg *api.GetInvestmentRequest) (*api.InvestmentResponse, error) {
	inst, err := s.repo.GetInvestment(ctx, msg.Id)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	if inst == nil {
		return nil, NotFound("investment %s not found", msg.Id)
	}
	return s.withPosition(ctx, inst)
}

func (s *InvestmentService) withPosition(ctx context.Context, inst *api.InvestmentResponse) (*api.InvestmentResponse, error) {
	pos, currentValue, unrealized, err := s.positionFor(ctx, inst)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	inst.Quantity = pos.Quantity
	inst.AvgCost = pos.AvgCost
	inst.CurrentPrice = effectivePrice(inst)
	inst.CurrentValue = currentValue
	inst.UnrealizedPnl = unrealized
	inst.RealizedPnl = pos.RealizedPnl
	return inst, nil
}

func (s *InvestmentService) ListInvestments(ctx context.Context, _ *api.ListInvestmentsRequest) (*api.ListInvestmentsResponse, error) {
	userID := ctx.Value(auth.UserIDKey).(string)
	instruments, err := s.repo.ListInvestments(ctx, userID)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	for i, inst := range instruments {
		filled, err := s.withPosition(ctx, inst)
		if err != nil {
			return nil, err
		}
		instruments[i] = filled
	}
	return &api.ListInvestmentsResponse{Investments: instruments}, nil
}

func (s *InvestmentService) UpdateInvestment(ctx context.Context, msg *api.UpdateInvestmentRequest) (*api.InvestmentResponse, error) {
	if msg.Name == "" {
		return nil, BadRequest("name is required")
	}
	if msg.InvestmentType != api.InvestmentType_INVESTMENT_TYPE_STOCK &&
		msg.InvestmentType != api.InvestmentType_INVESTMENT_TYPE_MUTUAL_FUND {
		return nil, BadRequest("INVESTMENT_TYPE must be STOCK or MUTUAL_FUND")
	}
	inst, err := s.repo.UpdateInvestment(ctx, msg.Id, msg.Name, msg.InvestmentType, msg.ManualNav)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	if inst == nil {
		return nil, NotFound("investment %s not found", msg.Id)
	}
	return s.withPosition(ctx, inst)
}

func (s *InvestmentService) DeleteInvestment(ctx context.Context, msg *api.DeleteInvestmentRequest) (*api.OperationResponse, error) {
	deleted, err := s.repo.DeleteInvestment(ctx, msg.Id)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	if !deleted {
		return nil, NotFound("investment %s not found", msg.Id)
	}
	return &api.OperationResponse{Success: true, Message: "investment deleted"}, nil
}

func (s *InvestmentService) AddLot(ctx context.Context, msg *api.AddLotRequest) (*api.LotResponse, error) {
	inst, err := s.repo.GetInvestment(ctx, msg.InvestmentId)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	if inst == nil {
		return nil, NotFound("investment %s not found", msg.InvestmentId)
	}
	if msg.Side != 1 && msg.Side != -1 {
		return nil, BadRequest("side must be 1 (buy) or -1 (sell)")
	}
	if msg.Quantity <= 0 {
		return nil, BadRequest("quantity must be greater than zero")
	}
	if msg.Price < 0 {
		return nil, BadRequest("price must not be negative")
	}
	if msg.Side == -1 {
		pos, _, _, err := s.positionFor(ctx, inst)
		if err != nil {
			return nil, ServerError("%v", err)
		}
		if msg.Quantity > pos.Quantity {
			return nil, BadRequest("cannot sell more than held")
		}
	}

	userID := ctx.Value(auth.UserIDKey).(string)
	lot, err := s.repo.CreateLot(ctx, userID, msg.InvestmentId, msg.Side, round2f(msg.Quantity), round2f(msg.Price), msg.OccurredAt)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	return lot, nil
}

func (s *InvestmentService) DeleteLot(ctx context.Context, msg *api.DeleteLotRequest) (*api.OperationResponse, error) {
	deleted, err := s.repo.DeleteLot(ctx, msg.Id)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	if !deleted {
		return nil, NotFound("lot %s not found", msg.Id)
	}
	return &api.OperationResponse{Success: true, Message: "lot deleted"}, nil
}

func (s *InvestmentService) ListLots(ctx context.Context, msg *api.GetInvestmentRequest) ([]*api.LotResponse, error) {
	inst, err := s.repo.GetInvestment(ctx, msg.Id)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	if inst == nil {
		return nil, NotFound("investment %s not found", msg.Id)
	}
	lots, err := s.repo.ListLots(ctx, msg.Id)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	return lots, nil
}

func (s *InvestmentService) SearchSymbols(ctx context.Context, msg *api.SearchSymbolsRequest) (*api.SearchSymbolsResponse, error) {
	if msg.Query == "" {
		return &api.SearchSymbolsResponse{}, nil
	}
	results, err := s.yahoo.Search(ctx, msg.Query)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	return &api.SearchSymbolsResponse{Results: results}, nil
}

func (s *InvestmentService) RefreshPrices(ctx context.Context, _ *api.RefreshPricesRequest) (*api.RefreshPricesResponse, error) {
	userID := ctx.Value(auth.UserIDKey).(string)
	instruments, err := s.repo.ListInvestments(ctx, userID)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	var symbols []string
	symIdx := map[string]string{}
	for _, inst := range instruments {
		if inst.Symbol != "" {
			symbols = append(symbols, inst.Symbol)
			symIdx[inst.Symbol] = inst.Id
		}
	}
	quotes, err := s.yahoo.GetQuotes(ctx, symbols)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	updated := 0
	for sym, quote := range quotes {
		if id, ok := symIdx[sym]; ok {
			if err := s.repo.UpdateQuote(ctx, id, float32(quote.Price), float32(quote.PrevClose)); err != nil {
				return nil, ServerError("%v", err)
			}
			updated++
		}
	}
	return &api.RefreshPricesResponse{Updated: int32(updated)}, nil
}

func (s *InvestmentService) RefreshAllPrices(ctx context.Context) error {
	instruments, err := s.repo.ListAllForRefresh(ctx)
	if err != nil {
		return err
	}
	bySymbol := map[string]string{}
	var symbols []string
	for _, inst := range instruments {
		if _, ok := bySymbol[inst.Symbol]; !ok {
			bySymbol[inst.Symbol] = inst.Id
			symbols = append(symbols, inst.Symbol)
		}
	}
	quotes, err := s.yahoo.GetQuotes(ctx, symbols)
	if err != nil {
		return err
	}
	for sym, quote := range quotes {
		if id, ok := bySymbol[sym]; ok {
			if err := s.repo.UpdateQuote(ctx, id, float32(quote.Price), float32(quote.PrevClose)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *InvestmentService) GetPortfolioSummary(ctx context.Context, _ *api.GetPortfolioSummaryRequest) (*api.PortfolioSummaryResponse, error) {
	userID := ctx.Value(auth.UserIDKey).(string)
	instruments, err := s.repo.ListInvestments(ctx, userID)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	var summary api.PortfolioSummaryResponse
	for _, inst := range instruments {
		pos, currentValue, _, err := s.positionFor(ctx, inst)
		if err != nil {
			return nil, ServerError("%v", err)
		}
		summary.TotalInvested += pos.CostBasis
		summary.TotalCurrentValue += currentValue
		summary.TotalUnrealizedPnl += (effectivePrice(inst) - pos.AvgCost) * pos.Quantity
		summary.TotalRealizedPnl += pos.RealizedPnl
	}
	return &summary, nil
}

type priceRange struct {
	rng      string
	interval string
	limit    int
	agg      string
}

var priceHistoryRanges = map[string]priceRange{
	"1d": {rng: "1d", interval: "15m"},
	"7d": {rng: "1mo", interval: "1d", limit: 7},
	"1m": {rng: "1mo", interval: "1d"},
	"6m": {rng: "6mo", interval: "1d"},
	"1y": {rng: "1y", interval: "1d", agg: "week"},
	"3y": {rng: "3y", interval: "1d", agg: "month"},
}

func aggregatePricePoints(points []PricePoint, agg string) []PricePoint {
	if len(points) == 0 || agg == "" {
		return points
	}
	type bucket struct {
		sum   float64
		n     int
		lastT int64
	}
	buckets := map[string]*bucket{}
	var order []string
	for _, p := range points {
		t := time.Unix(p.T, 0).UTC()
		var key string
		if agg == "week" {
			y, w := t.ISOWeek()
			key = fmt.Sprintf("%d-W%02d", y, w)
		} else {
			key = t.Format("2006-01")
		}
		b := buckets[key]
		if b == nil {
			b = &bucket{}
			buckets[key] = b
			order = append(order, key)
		}
		b.sum += p.Close
		b.n++
		b.lastT = p.T
	}
	out := make([]PricePoint, 0, len(order))
	for _, k := range order {
		b := buckets[k]
		out = append(out, PricePoint{T: b.lastT, Close: b.sum / float64(b.n)})
	}
	return out
}

func (s *InvestmentService) GetPriceHistory(ctx context.Context, investmentID, rangeID, from, to string, force bool) ([]PricePoint, error) {
	var cfg priceRange
	var cacheKey, rng string
	var period1, period2 int64

	if from != "" || to != "" {
		// Custom date range: fetch from Yahoo with explicit timestamps.
		f, err1 := time.Parse("2006-01-02", from)
		t, err2 := time.Parse("2006-01-02", to)
		if err1 != nil || err2 != nil {
			return nil, BadRequest("from and to must be YYYY-MM-DD")
		}
		if t.Before(f) || t.Sub(f) > 5*365*24*time.Hour {
			return nil, BadRequest("custom range must be at most 5 years")
		}
		period1 = f.Unix()
		period2 = t.Add(24 * time.Hour).Unix() // inclusive end-of-day
		cacheKey = "custom:" + from + ":" + to
		cfg = priceRange{interval: "1d"}
	} else {
		var ok bool
		cfg, ok = priceHistoryRanges[rangeID]
		if !ok {
			return nil, BadRequest("range must be one of 1d,7d,1m,6m,1y,3y or provide from/to")
		}
		cacheKey = rangeID
		rng = cfg.rng
	}

	inst, err := s.repo.GetInvestment(ctx, investmentID)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	if inst.Symbol == "" {
		return nil, nil
	}
	if !force {
		ts, closes, lastFetched, err := s.repo.GetPriceHistory(ctx, investmentID, cacheKey)
		if err != nil {
			return nil, ServerError("%v", err)
		}
		fresh := 6 * time.Hour
		if base, ok := priceHistoryFreshness[rangeID]; ok {
			fresh = base
		}
		if len(ts) > 0 && time.Now().Unix()-lastFetched < int64(fresh.Seconds()) {
			return toPricePoints(ts, closes), nil
		}
	}
	return s.fetchAndCachePriceHistory(ctx, inst.Id, inst.Symbol, cacheKey, cfg, rng, period1, period2)
}

var priceHistoryFreshness = map[string]time.Duration{
	"1d": 5 * time.Minute,
	"7d": time.Hour,
	"1m": time.Hour,
	"6m": 6 * time.Hour,
	"1y": 6 * time.Hour,
	"3y": 24 * time.Hour,
}

func toPricePoints(ts []int64, closes []float64) []PricePoint {
	points := make([]PricePoint, len(ts))
	for i := range ts {
		points[i] = PricePoint{T: ts[i], Close: closes[i]}
	}
	return points
}

func (s *InvestmentService) fetchAndCachePriceHistory(ctx context.Context, investmentID, symbol, rangeID string, cfg priceRange, rng string, period1, period2 int64) ([]PricePoint, error) {
	raw, err := s.yahoo.GetHistory(ctx, symbol, rng, cfg.interval, cfg.limit, period1, period2)
	if err != nil {
		return nil, ServerError("price history: %v", err)
	}
	points := aggregatePricePoints(raw, cfg.agg)
	ts := make([]int64, len(points))
	closes := make([]float64, len(points))
	for i, p := range points {
		ts[i] = p.T
		closes[i] = p.Close
	}
	if err := s.repo.UpsertPriceHistory(ctx, investmentID, rangeID, ts, closes, time.Now().Unix()); err != nil {
		logx.Debug("price history cache failed", "symbol", symbol, "range", rangeID, "err", err)
	}
	return points, nil
}

func (s *InvestmentService) RefreshAllPriceHistory(ctx context.Context) error {
	instruments, err := s.repo.ListAllForRefresh(ctx)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	for _, inst := range instruments {
		for rangeID, cfg := range priceHistoryRanges {
			ts, _, lastFetched, err := s.repo.GetPriceHistory(ctx, inst.Id, rangeID)
			if err != nil {
				continue
			}
			if len(ts) > 0 && now-lastFetched < int64(priceHistoryFreshness[rangeID].Seconds()) {
				continue
			}
			if _, err := s.fetchAndCachePriceHistory(ctx, inst.Id, inst.Symbol, rangeID, cfg, cfg.rng, 0, 0); err != nil {
				logx.Debug("price history refresh failed", "symbol", inst.Symbol, "range", rangeID, "err", err)
			}
		}
	}
	return nil
}