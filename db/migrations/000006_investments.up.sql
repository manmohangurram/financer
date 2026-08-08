-- Investments
CREATE TABLE IF NOT EXISTS investments (
  id              TEXT PRIMARY KEY,
  user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  symbol          TEXT,
  name            TEXT NOT NULL,
  investment_type INTEGER NOT NULL,
  current_price   REAL,
  prev_close      REAL,
  last_quote_at   TIMESTAMP,
  manual_nav      REAL,
  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(user_id, symbol)
);

-- Investment Lots
CREATE TABLE IF NOT EXISTS investment_lots (
  id            TEXT PRIMARY KEY,
  user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  investment_id TEXT NOT NULL REFERENCES investments(id) ON DELETE CASCADE,
  side          INTEGER NOT NULL,
  quantity      REAL NOT NULL,
  price         REAL NOT NULL,
  occurred_at   TIMESTAMP NOT NULL,
  created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Price History Cache
CREATE TABLE IF NOT EXISTS investment_price_history (
  investment_id TEXT NOT NULL,
  range_id      TEXT NOT NULL,
  t             INTEGER NOT NULL,
  close         REAL NOT NULL,
  fetched_at    INTEGER NOT NULL,
  PRIMARY KEY (investment_id, range_id, t)
);

CREATE INDEX IF NOT EXISTS idx_investments_user ON investments(user_id);
CREATE INDEX IF NOT EXISTS idx_lots_investment ON investment_lots(investment_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_price_history_lookup ON investment_price_history (investment_id, range_id);
