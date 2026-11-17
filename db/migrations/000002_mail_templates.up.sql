-- Learned parsing templates: one sender can have several guarded variants.
CREATE TABLE mail_templates (
  id              TEXT PRIMARY KEY,
  user_id         TEXT NOT NULL,
  sender          TEXT NOT NULL,
  name            TEXT NOT NULL,
  guards_json     TEXT NOT NULL DEFAULT '{}',
  patterns_json   TEXT NOT NULL,
  status          TEXT NOT NULL DEFAULT 'provisional',
  sample_count    INTEGER NOT NULL DEFAULT 1,
  successes       INTEGER NOT NULL DEFAULT 0,
  failures        INTEGER NOT NULL DEFAULT 0,
  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_used_at    TIMESTAMP,
  last_attempt_at TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_mail_templates_lookup ON mail_templates(user_id, sender, status);

-- Per-user LiteLLM settings. The API key uses the same encryption as mailbox
-- passwords (FINANCER_SECRET_KEY).
CREATE TABLE user_llm_settings (
  user_id                    TEXT PRIMARY KEY,
  base_url                   TEXT NOT NULL DEFAULT '',
  model                      TEXT NOT NULL DEFAULT '',
  api_key_enc                TEXT NOT NULL DEFAULT '',
  timeout_secs               INTEGER NOT NULL DEFAULT 20,
  enabled                    INTEGER NOT NULL DEFAULT 1,
  min_samples                INTEGER NOT NULL DEFAULT 3,
  failure_threshold          INTEGER NOT NULL DEFAULT 3,
  cooldown_minutes           INTEGER NOT NULL DEFAULT 60,
  auto_poll_throttle_minutes INTEGER NOT NULL DEFAULT 5,
  updated_at                 TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Provenance for a parsed message.
ALTER TABLE mail_messages ADD COLUMN template_id TEXT;

-- Rules now decide which mail matters; mailbox-level sender allowlists are gone.
ALTER TABLE mail_accounts DROP COLUMN sender_allowlist;
