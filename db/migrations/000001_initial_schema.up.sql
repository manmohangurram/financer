-- Financer schema baseline (Go backend).
-- One migration describes the complete final structure; there is no legacy
-- history to replay. Existing databases from older backends are not migrated:
-- delete the SQLite file (and -wal/-shm) and start fresh.

CREATE TABLE users (
  id            TEXT PRIMARY KEY,
  email         TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  name          TEXT,
  avatar_url    TEXT NOT NULL DEFAULT '',
  token_version INTEGER NOT NULL DEFAULT 0,
  created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE accounts (
  id             TEXT PRIMARY KEY,
  user_id        TEXT NOT NULL,
  bank_name      TEXT NOT NULL,
  nickname       TEXT,
  balance        REAL NOT NULL DEFAULT 0,
  type           TEXT NOT NULL,
  total_credit   REAL NOT NULL DEFAULT 0,
  total_debit    REAL NOT NULL DEFAULT 0,
  ending_numbers TEXT,
  created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_accounts_user_id ON accounts(user_id);
CREATE INDEX idx_accounts_user_ending ON accounts(user_id, ending_numbers);

CREATE TABLE transactions (
  id                 TEXT PRIMARY KEY,
  user_id            TEXT NOT NULL,
  account_id         TEXT NOT NULL,
  name               TEXT NOT NULL,
  clean_name         TEXT,
  amount             REAL NOT NULL,
  type               TEXT NOT NULL,
  occurred_at        TIMESTAMP NOT NULL,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  linked_transfer_id TEXT,
  external_id        TEXT,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
);
-- List/spending pagination, per-account totals, and the external-id dedupe key.
CREATE INDEX idx_transactions_user_occurred ON transactions(user_id, occurred_at DESC, id DESC);
CREATE INDEX idx_transactions_user_account_occurred ON transactions(user_id, account_id, occurred_at);
CREATE INDEX idx_transactions_account_id ON transactions(account_id);
CREATE UNIQUE INDEX idx_transactions_external_id ON transactions(user_id, external_id);

CREATE TABLE transfer_links (
  id                    TEXT PRIMARY KEY,
  user_id               TEXT NOT NULL,
  debit_transaction_id  TEXT NOT NULL,
  credit_transaction_id TEXT NOT NULL,
  created_at            TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (debit_transaction_id) REFERENCES transactions(id) ON DELETE CASCADE,
  FOREIGN KEY (credit_transaction_id) REFERENCES transactions(id) ON DELETE CASCADE
);
CREATE INDEX idx_transfer_links_user_id ON transfer_links(user_id);
CREATE INDEX idx_transfer_links_debit ON transfer_links(debit_transaction_id);
CREATE INDEX idx_transfer_links_credit ON transfer_links(credit_transaction_id);

CREATE TABLE categories (
  id         TEXT PRIMARY KEY,
  user_id    TEXT NOT NULL,
  name       TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  UNIQUE(user_id, name)
);

CREATE TABLE transaction_categories (
  transaction_id TEXT NOT NULL,
  category_id    TEXT NOT NULL,
  PRIMARY KEY (transaction_id, category_id),
  FOREIGN KEY (transaction_id) REFERENCES transactions(id) ON DELETE CASCADE,
  FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
);
CREATE INDEX idx_transaction_categories_category ON transaction_categories(category_id);

CREATE TABLE rules (
  id         TEXT PRIMARY KEY,
  user_id    TEXT NOT NULL,
  name       TEXT NOT NULL,
  priority   INTEGER NOT NULL DEFAULT 0,
  logic      TEXT NOT NULL DEFAULT 'OR',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_rules_user_priority ON rules(user_id, priority DESC, name);

CREATE TABLE rule_conditions (
  id          TEXT PRIMARY KEY,
  rule_id     TEXT NOT NULL,
  match_field TEXT NOT NULL,
  operator    TEXT NOT NULL,
  pattern     TEXT NOT NULL,
  FOREIGN KEY (rule_id) REFERENCES rules(id) ON DELETE CASCADE
);
CREATE INDEX idx_rule_conditions_rule ON rule_conditions(rule_id);

CREATE TABLE rule_actions (
  id                  TEXT PRIMARY KEY,
  rule_id             TEXT NOT NULL,
  action_type         TEXT NOT NULL DEFAULT 'SET_NAME',
  name_op             TEXT NOT NULL DEFAULT '',
  value               TEXT NOT NULL DEFAULT '',
  category_id         TEXT NOT NULL DEFAULT '',
  transfer_account_id TEXT NOT NULL DEFAULT '',
  FOREIGN KEY (rule_id) REFERENCES rules(id) ON DELETE CASCADE
);
CREATE INDEX idx_rule_actions_rule ON rule_actions(rule_id);

CREATE TABLE user_keys (
  id           TEXT PRIMARY KEY,
  user_id      TEXT NOT NULL,
  name         TEXT NOT NULL DEFAULT '',
  key_hash     TEXT NOT NULL,
  key_prefix   TEXT NOT NULL DEFAULT '',
  scope        TEXT NOT NULL DEFAULT 'read',
  created_at   TEXT NOT NULL,
  expires_at   TEXT,
  last_used_at TEXT,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_user_keys_user ON user_keys(user_id);
CREATE UNIQUE INDEX idx_user_keys_key_hash ON user_keys(key_hash);

CREATE TABLE mail_accounts (
  id               TEXT PRIMARY KEY,
  user_id          TEXT NOT NULL,
  label            TEXT NOT NULL DEFAULT '',
  imap_host        TEXT NOT NULL,
  imap_port        INTEGER NOT NULL,
  username         TEXT NOT NULL,
  secret_enc       TEXT NOT NULL,
  enabled          INTEGER NOT NULL DEFAULT 1,
  last_polled_at   TIMESTAMP,
  last_error       TEXT NOT NULL DEFAULT '',
  created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_mail_accounts_user_id ON mail_accounts(user_id);

-- Learned parsing templates: one sender can have several guarded variants.
CREATE TABLE mail_templates (
  id             TEXT PRIMARY KEY,
  user_id        TEXT NOT NULL,
  sender         TEXT NOT NULL,
  name           TEXT NOT NULL,
  guards_json    TEXT NOT NULL DEFAULT '{}',
  patterns_json  TEXT NOT NULL,
  status         TEXT NOT NULL DEFAULT 'provisional',
  sample_count   INTEGER NOT NULL DEFAULT 1,
  successes      INTEGER NOT NULL DEFAULT 0,
  failures       INTEGER NOT NULL DEFAULT 0,
  created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_used_at   TIMESTAMP,
  last_attempt_at TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_mail_templates_lookup ON mail_templates(user_id, sender, status);

-- Per-user LiteLLM settings. The API key uses the same encryption as mailbox
-- passwords (FINANCER_SECRET_KEY).
CREATE TABLE user_llm_settings (
  user_id                     TEXT PRIMARY KEY,
  base_url                    TEXT NOT NULL DEFAULT '',
  model                       TEXT NOT NULL DEFAULT '',
  api_key_enc                 TEXT NOT NULL DEFAULT '',
  timeout_secs                INTEGER NOT NULL DEFAULT 20,
  enabled                     INTEGER NOT NULL DEFAULT 1,
  min_samples                 INTEGER NOT NULL DEFAULT 3,
  failure_threshold           INTEGER NOT NULL DEFAULT 3,
  cooldown_minutes            INTEGER NOT NULL DEFAULT 60,
  auto_poll_throttle_minutes  INTEGER NOT NULL DEFAULT 5,
  updated_at                  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE mail_rules (
  id               TEXT PRIMARY KEY,
  user_id          TEXT NOT NULL,
  position         INTEGER NOT NULL DEFAULT 0,
  enabled          INTEGER NOT NULL DEFAULT 1,
  from_pattern     TEXT NOT NULL DEFAULT '',
  subject_contains TEXT NOT NULL DEFAULT '',
  body_contains    TEXT NOT NULL DEFAULT '',
  action           TEXT NOT NULL DEFAULT 'review',
  account_id       TEXT,
  category_ids     TEXT NOT NULL DEFAULT '[]',
  created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE SET NULL
);
CREATE INDEX idx_mail_rules_user_position ON mail_rules(user_id, position);

CREATE TABLE mail_messages (
  id               TEXT PRIMARY KEY,
  user_id          TEXT NOT NULL,
  mail_account_id  TEXT NOT NULL,
  message_id       TEXT NOT NULL,
  from_addr        TEXT NOT NULL DEFAULT '',
  received_at      TIMESTAMP,
  subject          TEXT NOT NULL DEFAULT '',
  raw_text         TEXT,
  parsed_json      TEXT,
  approved_json    TEXT,
  status           TEXT NOT NULL,
  extractor_source TEXT NOT NULL DEFAULT '',
  confidence       REAL,
  transaction_id   TEXT,
  template_id      TEXT,
  created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (mail_account_id) REFERENCES mail_accounts(id) ON DELETE CASCADE,
  FOREIGN KEY (transaction_id) REFERENCES transactions(id) ON DELETE SET NULL,
  FOREIGN KEY (template_id) REFERENCES mail_templates(id) ON DELETE SET NULL,
  UNIQUE(user_id, message_id)
);
CREATE INDEX idx_mail_messages_user_status ON mail_messages(user_id, status);
CREATE INDEX idx_mail_messages_user_created ON mail_messages(user_id, created_at DESC);
