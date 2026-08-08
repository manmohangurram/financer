package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mohan9182/financer/logx"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mohan9182/financer/api"
)

const yahooBaseURL = "https://query1.finance.yahoo.com"

type YahooClient struct {
	baseURL string
	client  *http.Client
}

func NewYahooClient() *YahooClient {
	return &YahooClient{baseURL: yahooBaseURL, client: &http.Client{Timeout: 10 * time.Second}}
}

func (c *YahooClient) get(ctx context.Context, u string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36")
	return c.client.Do(req)
}

type YahooQuote struct {
	Price     float64
	PrevClose float64
}

func (c *YahooClient) Search(ctx context.Context, query string) ([]*api.SymbolResult, error) {
	u := fmt.Sprintf("%s/v1/finance/search?q=%s&quotesCount=8&newsCount=0", c.baseURL, url.QueryEscape(query))
	resp, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo search status %d", resp.StatusCode)
	}
	var payload struct {
		Quotes []struct {
			Symbol    string `json:"symbol"`
			Shortname string `json:"shortname"`
			Longname  string `json:"longname"`
			QuoteType string `json:"quoteType"`
		} `json:"quotes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	var out []*api.SymbolResult
	for _, q := range payload.Quotes {
		out = append(out, &api.SymbolResult{
			Symbol:         q.Symbol,
			Name:           firstNonEmpty(q.Shortname, q.Longname, q.Symbol),
			InvestmentType: yahooQuoteType(q.QuoteType),
		})
	}
	return out, nil
}

func (c *YahooClient) GetQuotes(ctx context.Context, symbols []string) (map[string]YahooQuote, error) {
	out := make(map[string]YahooQuote, len(symbols))
	for _, sym := range symbols {
		q, ok := c.fetchQuote(ctx, sym)
		if !ok && !strings.HasSuffix(sym, ".NS") {
			logx.Debug("quote fallback to .NS", "symbol", sym)
			q, ok = c.fetchQuote(ctx, sym+".NS")
		}
		if ok {
			out[sym] = q
		} else {
			logx.Debug("quote skipped: no valid price data", "symbol", sym)
		}
	}
	return out, nil
}

func (c *YahooClient) fetchQuote(ctx context.Context, sym string) (YahooQuote, bool) {
	u := fmt.Sprintf("%s/v8/finance/chart/%s?interval=1d&range=1d", c.baseURL, url.PathEscape(sym))
	resp, err := c.get(ctx, u)
	if err != nil {
		logx.Debug("quote fetch failed", "symbol", sym, "err", err)
		return YahooQuote{}, false
	}
	var payload struct {
		Chart struct {
			Result []struct {
				Meta struct {
					RegularMarketPrice         float64 `json:"regularMarketPrice"`
					ChartPreviousClose         float64 `json:"chartPreviousClose"`
					RegularMarketPreviousClose float64 `json:"regularMarketPreviousClose"`
				} `json:"meta"`
			} `json:"result"`
		} `json:"chart"`
	}
	decodeErr := json.NewDecoder(resp.Body).Decode(&payload)
	resp.Body.Close()
	if decodeErr != nil || len(payload.Chart.Result) == 0 || payload.Chart.Result[0].Meta.RegularMarketPrice == 0 {
		return YahooQuote{}, false
	}
	prev := payload.Chart.Result[0].Meta.RegularMarketPreviousClose
	if prev == 0 {
		prev = payload.Chart.Result[0].Meta.ChartPreviousClose
	}
	return YahooQuote{Price: payload.Chart.Result[0].Meta.RegularMarketPrice, PrevClose: prev}, true
}

func yahooQuoteType(qt string) api.InvestmentType {
	lower := strings.ToLower(qt)
	if strings.Contains(lower, "etf") || strings.Contains(lower, "fund") {
		return api.InvestmentType_INVESTMENT_TYPE_MUTUAL_FUND
	}
	return api.InvestmentType_INVESTMENT_TYPE_STOCK
}

type PricePoint struct {
	T     int64   `json:"t"`
	Close float64 `json:"close"`
}

// GetHistory fetches daily/intraday chart data. When period1/period2 are set
// (Unix seconds) a custom date range is used; otherwise the preset `rng`
// (e.g. "1mo") is requested. Yahoo needs the exchange suffix for most
// non-US symbols, so a bare symbol is retried with ".NS" on empty/error.
func (c *YahooClient) GetHistory(ctx context.Context, symbol, rng, interval string, limit int, period1, period2 int64) ([]PricePoint, error) {
	points, err := c.fetchHistory(ctx, symbol, rng, interval, limit, period1, period2)
	if (len(points) == 0 || err != nil) && !strings.HasSuffix(symbol, ".NS") {
		return c.fetchHistory(ctx, symbol+".NS", rng, interval, limit, period1, period2)
	}
	return points, err
}

func (c *YahooClient) fetchHistory(ctx context.Context, symbol, rng, interval string, limit int, period1, period2 int64) ([]PricePoint, error) {
	u := fmt.Sprintf("%s/v8/finance/chart/%s?interval=%s", c.baseURL, url.PathEscape(symbol), interval)
	if period1 > 0 {
		u += fmt.Sprintf("&period1=%d&period2=%d", period1, period2)
	} else {
		u += "&range=" + rng
	}
	resp, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("yahoo chart %s status %d", symbol, resp.StatusCode)
	}
	var payload struct {
		Chart struct {
			Result []struct {
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Close []*float64 `json:"close"`
					} `json:"quote"`
				} `json:"indicators"`
			} `json:"result"`
		} `json:"chart"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if len(payload.Chart.Result) == 0 {
		return nil, nil
	}
	res := payload.Chart.Result[0]
	var closes []*float64
	if len(res.Indicators.Quote) > 0 {
		closes = res.Indicators.Quote[0].Close
	}
	var points []PricePoint
	for i, ts := range res.Timestamp {
		if i >= len(closes) || closes[i] == nil {
			continue
		}
		points = append(points, PricePoint{T: ts, Close: *closes[i]})
	}
	if limit > 0 && len(points) > limit {
		points = points[len(points)-limit:]
	}
	return points, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
