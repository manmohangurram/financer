package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mohan9182/financer/api"
	"github.com/mohan9182/financer/logx"
)

// yahooConfig makes the Yahoo endpoints configurable so a dead base URL can be
// swapped without a code change. Paths use {placeholder} tokens:
// {symbol}, {interval}, {range}, {period1}, {period2}, {query}.
type yahooConfig struct {
	Bases  []string `json:"bases"`
	Chart  string   `json:"chart"`
	Search string   `json:"search"`
}

// yahooConfigPath returns the config path: FINANCER_YAHOO_CONFIG or the repo
// default config/yahoo.json.
func yahooConfigPath() string {
	if p := os.Getenv("FINANCER_YAHOO_CONFIG"); p != "" {
		return p
	}
	return filepath.Join("config", "yahoo.json")
}

func loadYahooConfig() (yahooConfig, error) {
	return loadYahooConfigFrom(yahooConfigPath())
}

func loadYahooConfigFrom(path string) (yahooConfig, error) {
	var cfg yahooConfig
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("yahoo config: %w", err)
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("yahoo config: %w", err)
	}
	if len(cfg.Bases) == 0 || cfg.Chart == "" || cfg.Search == "" {
		return cfg, fmt.Errorf("yahoo config: bases, chart, and search are required")
	}
	return cfg, nil
}

type YahooClient struct {
	bases  []string
	chart  string
	search string
	client *http.Client
}

func NewYahooClient() (*YahooClient, error) {
	cfg, err := loadYahooConfig()
	if err != nil {
		return nil, err
	}
	return &YahooClient{bases: cfg.Bases, chart: cfg.Chart, search: cfg.Search, client: &http.Client{Timeout: 10 * time.Second}}, nil
}

func (c *YahooClient) get(ctx context.Context, u string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36")
	return c.client.Do(req)
}

// buildURL fills {placeholder} tokens in a path template for a given base and
// strips any tokens that were not provided.
func buildURL(base, template string, params map[string]string) string {
	u := base + template
	for k, v := range params {
		u = strings.ReplaceAll(u, "{"+k+"}", url.QueryEscape(v))
	}
	u = placeholderRE.ReplaceAllString(u, "")
	return u
}

var placeholderRE = regexp.MustCompile(`\{[a-z0-9]+\}`)

// symbolCandidates returns [symbol, symbol+".NS"] for bare symbols so the
// exchange suffix can be tried, unless the symbol already has one.
func symbolCandidates(symbol string) []string {
	if strings.Contains(symbol, ".") {
		return []string{symbol}
	}
	return []string{symbol, symbol + ".NS"}
}

type YahooQuote struct {
	Price     float64
	PrevClose float64
}

func (c *YahooClient) Search(ctx context.Context, query string) ([]*api.SymbolResult, error) {
	var lastErr error
	for _, base := range c.bases {
		u := buildURL(base, c.search, map[string]string{"query": query})
		results, err := c.trySearch(ctx, u)
		if err == nil {
			return results, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (c *YahooClient) trySearch(ctx context.Context, u string) ([]*api.SymbolResult, error) {
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
		if ok {
			out[sym] = q
		} else {
			logx.Debug("quote skipped: no valid price data", "symbol", sym)
		}
	}
	return out, nil
}

// fetchQuote tries each configured base and, for bare symbols, the .NS
// exchange suffix until a valid quote is found.
func (c *YahooClient) fetchQuote(ctx context.Context, sym string) (YahooQuote, bool) {
	now := time.Now()
	p1, p2 := now.Add(-24*time.Hour).Unix(), now.Unix()
	for _, base := range c.bases {
		for _, cand := range symbolCandidates(sym) {
			u := buildURL(base, c.chart, map[string]string{
				"symbol": cand, "interval": "1d", "period1": fmt.Sprintf("%d", p1), "period2": fmt.Sprintf("%d", p2),
			})
			resp, err := c.get(ctx, u)
			if err != nil {
				logx.Debug("quote fetch failed", "symbol", cand, "err", err)
				continue
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
				continue
			}
			prev := payload.Chart.Result[0].Meta.RegularMarketPreviousClose
			if prev == 0 {
				prev = payload.Chart.Result[0].Meta.ChartPreviousClose
			}
			return YahooQuote{Price: payload.Chart.Result[0].Meta.RegularMarketPrice, PrevClose: prev}, true
		}
	}
	return YahooQuote{}, false
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

// GetHistory fetches daily/intraday chart data for the period1..period2 Unix
// seconds (period-based, matching Yahoo's chart requests). Tries each configured
// base and, for bare symbols, the .NS exchange suffix.
func (c *YahooClient) GetHistory(ctx context.Context, symbol, interval string, limit int, period1, period2 int64) ([]PricePoint, error) {
	var lastErr error
	for _, base := range c.bases {
		for _, cand := range symbolCandidates(symbol) {
			points, err := c.fetchHistory(ctx, base, cand, interval, limit, period1, period2)
			if err == nil {
				if len(points) > 0 || cand == symbol {
					return points, nil
				}
				// empty result for the bare symbol — try the next candidate
			} else {
				lastErr = err
			}
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no price data for %s", symbol)
	}
	return nil, lastErr
}

func (c *YahooClient) fetchHistory(ctx context.Context, base, symbol, interval string, limit int, period1, period2 int64) ([]PricePoint, error) {
	params := map[string]string{
		"symbol":   symbol,
		"interval": interval,
		"period1":  fmt.Sprintf("%d", period1),
		"period2":  fmt.Sprintf("%d", period2),
	}
	u := buildURL(base, c.chart, params)
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
