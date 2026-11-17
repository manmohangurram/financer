-- Query-shape indexes. The schema already indexes each foreign key on its own;
-- these composites match the actual ORDER BY / filter combinations and remove
-- full scans on the join table.

-- Transaction list: WHERE user_id = ? ORDER BY occurred_at DESC, id DESC, and
-- spending range scans (user_id + occurred_at).
CREATE INDEX IF NOT EXISTS idx_transactions_user_occurred
  ON transactions(user_id, occurred_at DESC, id DESC);

-- Account-filtered list/spending: WHERE user_id = ? AND account_id = ?
-- ORDER BY occurred_at.
CREATE INDEX IF NOT EXISTS idx_transactions_user_account_occurred
  ON transactions(user_id, account_id, occurred_at);

-- Category filter runs an EXISTS on category_id; the composite primary key
-- starts with transaction_id, so category_id lookups need their own index.
CREATE INDEX IF NOT EXISTS idx_transaction_categories_category
  ON transaction_categories(category_id);

-- Inbox list: WHERE user_id = ? ORDER BY created_at DESC.
CREATE INDEX IF NOT EXISTS idx_mail_messages_user_created
  ON mail_messages(user_id, created_at DESC);

-- Rule list: WHERE user_id = ? ORDER BY priority DESC, name ASC.
CREATE INDEX IF NOT EXISTS idx_rules_user_priority
  ON rules(user_id, priority DESC, name);
