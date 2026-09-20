//! Yahoo Finance client — mirrors Go's `services/yahoo.go`: config-driven base
//! URLs with `{placeholder}` tokens, retry across bases, `.NS` suffix fallback.

use serde::Deserialize;

use crate::config::YahooConfigSection;
use crate::repo::traits::investment::InvestmentType;

/// A quote for one symbol.
#[derive(Debug, Clone, Copy)]
pub struct Quote {
    pub price: f64,
    pub prev_close: f64,
}

/// A chart price point (Go's `PricePoint`).
#[derive(Debug, Clone, Copy, serde::Serialize)]
pub struct PricePoint {
    pub t: i64,
    pub close: f64,
}

#[derive(Debug, Clone)]
pub struct SymbolResult {
    pub symbol: String,
    pub name: String,
    pub investment_type: InvestmentType,
}

#[derive(Clone)]
pub struct YahooClient {
    bases: Vec<String>,
    chart: String,
    search: String,
    http: reqwest::Client,
}

impl YahooClient {
    pub fn new(cfg: &YahooConfigSection) -> anyhow::Result<Self> {
        if cfg.bases.is_empty() || cfg.chart.is_empty() || cfg.search.is_empty() {
            anyhow::bail!("yahoo config: bases, chart, and search are required");
        }
        Ok(Self {
            bases: cfg.bases.clone(),
            chart: cfg.chart.clone(),
            search: cfg.search.clone(),
            http: reqwest::Client::builder()
                .timeout(std::time::Duration::from_secs(10))
                .build()?,
        })
    }

    /// Fill `{placeholder}` tokens; strip any tokens that were not provided.
    fn build_url(base: &str, template: &str, params: &[(&str, &str)]) -> String {
        let mut u = format!("{base}{template}");
        for (k, v) in params {
            u = u.replace(&format!("{{{k}}}"), &urlencode(v));
        }
        // strip remaining {tokens}
        let mut out = String::with_capacity(u.len());
        let mut in_brace = false;
        for c in u.chars() {
            match c {
                '{' => in_brace = true,
                '}' => in_brace = false,
                _ if !in_brace => out.push(c),
                _ => {}
            }
        }
        out
    }

    async fn get(&self, url: &str) -> anyhow::Result<reqwest::Response> {
        self.http
            .get(url)
            .header("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36")
            .send()
            .await
            .map_err(|e| anyhow::anyhow!(e))
    }

    /// Search symbols by query.
    pub async fn search(&self, query: &str) -> anyhow::Result<Vec<SymbolResult>> {
        let mut last_err: Option<anyhow::Error> = None;
        for base in &self.bases {
            let url = Self::build_url(base, &self.search, &[("query", query)]);
            match self.try_search(&url).await {
                Ok(r) => return Ok(r),
                Err(e) => last_err = Some(e),
            }
        }
        Err(last_err.unwrap_or_else(|| anyhow::anyhow!("symbol search unavailable")))
    }

    async fn try_search(&self, url: &str) -> anyhow::Result<Vec<SymbolResult>> {
        let resp = self.get(url).await?;
        if resp.status() != reqwest::StatusCode::OK {
            anyhow::bail!("yahoo search status {}", resp.status());
        }
        let payload: SearchPayload = resp.json().await?;
        Ok(payload
            .quotes
            .into_iter()
            .map(|q| SymbolResult {
                symbol: q.symbol.clone(),
                name: q
                    .shortname
                    .filter(|s| !s.is_empty())
                    .or(q.longname.filter(|s| !s.is_empty()))
                    .unwrap_or(q.symbol),
                investment_type: yahoo_quote_type(q.quotetype.as_deref().unwrap_or("")),
            })
            .collect())
    }

    /// Fetch quotes for symbols (map keyed by original symbol).
    pub async fn get_quotes(&self, symbols: &[String]) -> std::collections::HashMap<String, Quote> {
        let mut out = std::collections::HashMap::new();
        for sym in symbols {
            if let Some(q) = self.fetch_quote(sym).await {
                out.insert(sym.clone(), q);
            }
        }
        out
    }

    /// Try each base and, for bare symbols, the `.NS` suffix.
    async fn fetch_quote(&self, sym: &str) -> Option<Quote> {
        let now = crate::utils::timex::now_utc().timestamp();
        let p1 = now - 24 * 3600;
        let p2 = now;
        for base in &self.bases {
            for cand in symbol_candidates(sym) {
                let url = Self::build_url(
                    base,
                    &self.chart,
                    &[
                        ("symbol", &cand),
                        ("interval", "1d"),
                        ("period1", &p1.to_string()),
                        ("period2", &p2.to_string()),
                    ],
                );
                let Ok(resp) = self.get(&url).await else {
                    continue;
                };
                let Ok(payload) = resp.json::<ChartPayload>().await else {
                    continue;
                };
                let Some(meta) = payload.chart.result.first().map(|r| &r.meta) else {
                    continue;
                };
                if meta.regular_market_price == 0.0 {
                    continue;
                }
                let prev = if meta.regular_market_previous_close == 0.0 {
                    meta.chart_previous_close
                } else {
                    meta.regular_market_previous_close
                };
                return Some(Quote {
                    price: meta.regular_market_price,
                    prev_close: prev,
                });
            }
        }
        None
    }

    /// Fetch chart history for period1..period2 Unix seconds.
    pub async fn get_history(
        &self,
        symbol: &str,
        interval: &str,
        limit: usize,
        period1: i64,
        period2: i64,
    ) -> anyhow::Result<Vec<PricePoint>> {
        let mut last_err: Option<anyhow::Error> = None;
        for base in &self.bases {
            for cand in symbol_candidates(symbol) {
                match self
                    .fetch_history(base, &cand, interval, limit, period1, period2)
                    .await
                {
                    Ok(points) => {
                        if !points.is_empty() || cand == symbol {
                            return Ok(points);
                        }
                    }
                    Err(e) => last_err = Some(e),
                }
            }
        }
        Err(last_err.unwrap_or_else(|| anyhow::anyhow!("no price data for {symbol}")))
    }

    async fn fetch_history(
        &self,
        base: &str,
        symbol: &str,
        interval: &str,
        limit: usize,
        period1: i64,
        period2: i64,
    ) -> anyhow::Result<Vec<PricePoint>> {
        let url = Self::build_url(
            base,
            &self.chart,
            &[
                ("symbol", symbol),
                ("interval", interval),
                ("period1", &period1.to_string()),
                ("period2", &period2.to_string()),
            ],
        );
        let resp = self.get(&url).await?;
        if resp.status() != reqwest::StatusCode::OK {
            anyhow::bail!("yahoo chart {symbol} status {}", resp.status());
        }
        let payload: ChartPayload = resp.json().await?;
        let Some(res) = payload.chart.result.first() else {
            return Ok(Vec::new());
        };
        let closes = res
            .indicators
            .quote
            .first()
            .map(|q| &q.close)
            .cloned()
            .unwrap_or_default();
        let mut points = Vec::new();
        for (i, ts) in res.timestamp.iter().enumerate() {
            if let Some(Some(c)) = closes.get(i) {
                points.push(PricePoint { t: *ts, close: *c });
            }
        }
        if limit > 0 && points.len() > limit {
            points.drain(..points.len() - limit);
        }
        Ok(points)
    }
}

fn symbol_candidates(symbol: &str) -> Vec<String> {
    if symbol.contains('.') {
        vec![symbol.to_string()]
    } else {
        vec![symbol.to_string(), format!("{symbol}.NS")]
    }
}

fn yahoo_quote_type(qt: &str) -> InvestmentType {
    let lower = qt.to_lowercase();
    if lower.contains("etf") || lower.contains("fund") {
        InvestmentType::MutualFund
    } else {
        InvestmentType::Stock
    }
}

fn urlencode(v: &str) -> String {
    use std::fmt::Write as _;
    // Percent-encode for query strings; reqwest would otherwise need it.
    let mut out = String::with_capacity(v.len());
    for b in v.bytes() {
        match b {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'_' | b'.' | b'~' => {
                out.push(b as char);
            }
            _ => {
                out.push('%');
                let _ = write!(out, "{b:02X}");
            }
        }
    }
    out
}

#[derive(Debug, Deserialize)]
struct SearchPayload {
    quotes: Vec<SearchQuote>,
}

#[derive(Debug, Deserialize)]
struct SearchQuote {
    symbol: String,
    shortname: Option<String>,
    longname: Option<String>,
    quotetype: Option<String>,
}

#[derive(Debug, Deserialize)]
struct ChartPayload {
    chart: Chart,
}

#[derive(Debug, Deserialize)]
struct Chart {
    result: Vec<ChartResult>,
}

#[derive(Debug, Deserialize)]
struct ChartResult {
    meta: Meta,
    timestamp: Vec<i64>,
    indicators: Indicators,
}

#[derive(Debug, Deserialize)]
struct Meta {
    #[serde(rename = "regularMarketPrice")]
    regular_market_price: f64,
    #[serde(rename = "chartPreviousClose")]
    chart_previous_close: f64,
    #[serde(rename = "regularMarketPreviousClose")]
    regular_market_previous_close: f64,
}

#[derive(Debug, Deserialize)]
struct Indicators {
    quote: Vec<QuoteIndicator>,
}

#[derive(Debug, Deserialize)]
struct QuoteIndicator {
    close: Vec<Option<f64>>,
}
