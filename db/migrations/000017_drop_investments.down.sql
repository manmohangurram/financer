-- Restores the shape only — the rows are gone. Kept so a migrate-down does not
-- fail, not so the feature can come back from here.
CREATE TABLE IF NOT EXISTS investments (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  symbol TEXT NOT NULL,
  name TEXT NOT NULL,
  investment_type INTEGER NOT NULL,
  current_price REAL,
  prev_close REAL,
  last_quote_at TIMESTAMP,
  manual_nav REAL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS investment_lots (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  investment_id TEXT NOT NULL REFERENCES investments(id) ON DELETE CASCADE,
  side INTEGER NOT NULL,
  quantity REAL NOT NULL,
  price REAL NOT NULL,
  occurred_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  external_id TEXT
);
CREATE TABLE IF NOT EXISTS investment_price_history (
  investment_id TEXT NOT NULL REFERENCES investments(id) ON DELETE CASCADE,
  range_id TEXT NOT NULL,
  fetched_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  points_json TEXT NOT NULL,
  PRIMARY KEY (investment_id, range_id)
);
CREATE INDEX IF NOT EXISTS idx_investments_user ON investments(user_id);
CREATE INDEX IF NOT EXISTS idx_lots_investment ON investment_lots(investment_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_price_history_lookup ON investment_price_history (investment_id, range_id);
