-- Per-user API keys for external tool / MCP auth (issue #60).
CREATE TABLE IF NOT EXISTS user_keys (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  key_hash TEXT NOT NULL,
  key_prefix TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  expires_at TEXT,
  last_used_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_user_keys_user ON user_keys(user_id);
-- Auth lookup is by key_hash; keep it unique so it stays fast and collision-free.
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_keys_key_hash ON user_keys(key_hash);
