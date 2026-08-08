-- Categories
CREATE TABLE IF NOT EXISTS categories (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  name TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  UNIQUE(user_id, name)
);

-- Transaction Categories
CREATE TABLE IF NOT EXISTS transaction_categories (
  transaction_id TEXT NOT NULL,
  category_id TEXT NOT NULL,
  PRIMARY KEY (transaction_id, category_id),
  FOREIGN KEY (transaction_id) REFERENCES transactions(id) ON DELETE CASCADE,
  FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
);

-- Aliases (rules)
CREATE TABLE IF NOT EXISTS rules (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  name TEXT NOT NULL,
  priority INTEGER NOT NULL DEFAULT 0,
  logic TEXT NOT NULL DEFAULT 'OR',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Rule Conditions
CREATE TABLE IF NOT EXISTS rule_conditions (
  id TEXT PRIMARY KEY,
  rule_id TEXT NOT NULL,
  match_field TEXT NOT NULL,
  operator TEXT NOT NULL,
  pattern TEXT NOT NULL,
  FOREIGN KEY (rule_id) REFERENCES rules(id) ON DELETE CASCADE
);

-- Rule Actions
CREATE TABLE IF NOT EXISTS rule_actions (
  id TEXT PRIMARY KEY,
  rule_id TEXT NOT NULL,
  action_type TEXT NOT NULL DEFAULT 'SET_NAME',
  name_op TEXT NOT NULL DEFAULT '',
  value TEXT NOT NULL DEFAULT '',
  category_id TEXT NOT NULL DEFAULT '',
  transfer_account_id TEXT NOT NULL DEFAULT '',
  FOREIGN KEY (rule_id) REFERENCES rules(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_categories_user_id ON categories(user_id);
CREATE INDEX IF NOT EXISTS idx_rules_user_id ON rules(user_id);
