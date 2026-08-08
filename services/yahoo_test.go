package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestYahooSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"quotes":[
			{"symbol":"RELIANCE.NS","shortname":"Reliance Industries","longname":"","quoteType":"EQUITY"},
			{"symbol":"TCS.NS","shortname":"","longname":"Tata Consultancy Services","quoteType":"EQUITY"},
			{"symbol":"HDFC.NS","shortname":"","longname":"","quoteType":"ETF"}
		]}`))
	}))
	defer srv.Close()

	c := &YahooClient{baseURL: srv.URL, client: srv.Client()}
	res, err := c.Search(context.Background(), "reliance")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 3 {
		t.Fatalf("len = %d, want 3", len(res))
	}
	if res[0].Symbol != "RELIANCE.NS" || res[0].Name != "Reliance Industries" {
		t.Errorf("first result = %+v", res[0])
	}
	// ETF maps to MUTUAL_FUND
	if res[2].InvestmentType != 2 {
		t.Errorf("ETF type = %v, want MUTUAL_FUND(2)", res[2].InvestmentType)
	}
}

func TestYahooGetQuotes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v8/finance/chart/RELIANCE.NS":
			w.Write([]byte(`{"chart":{"result":[{"meta":{"regularMarketPrice":2980.5,"regularMarketPreviousClose":2950.0}}]}}`))
		case "/v8/finance/chart/TCS.NS":
			w.Write([]byte(`{"chart":{"result":[{"meta":{"regularMarketPrice":0}}]}}`))
		case "/v8/finance/chart/MISSING.NS":
			http.Error(w, `{"chart":{"result":null,"error":{"code":"Not Found"}}}`, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &YahooClient{baseURL: srv.URL, client: srv.Client()}
	q, err := c.GetQuotes(context.Background(), []string{"RELIANCE.NS", "TCS.NS", "MISSING.NS"})
	if err != nil {
		t.Fatal(err)
	}
	// TCS price 0 and MISSING (HTTP 404, no result) are filtered out
	if len(q) != 1 {
		t.Fatalf("len = %d, want 1", len(q))
	}
	if q["RELIANCE.NS"].Price != 2980.5 {
		t.Errorf("price = %v, want 2980.5", q["RELIANCE.NS"].Price)
	}
	if q["RELIANCE.NS"].PrevClose != 2950.0 {
		t.Errorf("prev close = %v, want 2950.0", q["RELIANCE.NS"].PrevClose)
	}
}
