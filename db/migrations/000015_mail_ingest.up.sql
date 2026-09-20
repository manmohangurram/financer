-- Linked mailboxes: credentials for IMAP ingest. One row per mailbox.
CREATE TABLE IF NOT EXISTS mail_accounts (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  label TEXT NOT NULL DEFAULT '',
  imap_host TEXT NOT NULL,
  imap_port INTEGER NOT NULL,
  username TEXT NOT NULL,
  secret_enc TEXT NOT NULL,
  sender_allowlist TEXT NOT NULL DEFAULT '[]',
  enabled INTEGER NOT NULL DEFAULT 1,
  last_polled_at TIMESTAMP,
  last_error TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- How fetched mail is disposed: first match by position wins.
CREATE TABLE IF NOT EXISTS mail_rules (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  position INTEGER NOT NULL DEFAULT 0,
  enabled INTEGER NOT NULL DEFAULT 1,
  from_pattern TEXT NOT NULL DEFAULT '',
  subject_contains TEXT NOT NULL DEFAULT '',
  body_contains TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL DEFAULT 'review',
  account_id TEXT,
  category_ids TEXT NOT NULL DEFAULT '[]',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE SET NULL
);

-- One row per seen email. Kept after resolution as the idempotency tombstone.
CREATE TABLE IF NOT EXISTS mail_messages (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  mail_account_id TEXT NOT NULL,
  message_id TEXT NOT NULL,
  from_addr TEXT NOT NULL DEFAULT '',
  received_at TIMESTAMP,
  subject TEXT NOT NULL DEFAULT '',
  raw_text TEXT,
  parsed_json TEXT,
  approved_json TEXT,
  status TEXT NOT NULL,
  extractor_source TEXT NOT NULL DEFAULT '',
  confidence REAL,
  transaction_id TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (mail_account_id) REFERENCES mail_accounts(id) ON DELETE CASCADE,
  FOREIGN KEY (transaction_id) REFERENCES transactions(id) ON DELETE SET NULL,
  UNIQUE(user_id, message_id)
);

CREATE INDEX IF NOT EXISTS idx_mail_accounts_user_id ON mail_accounts(user_id);
CREATE INDEX IF NOT EXISTS idx_mail_rules_user_position ON mail_rules(user_id, position);
CREATE INDEX IF NOT EXISTS idx_mail_messages_user_status ON mail_messages(user_id, status);
