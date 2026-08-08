-- Per-account cached credit/debit totals (dashboard + per-account analysis).
ALTER TABLE accounts ADD COLUMN total_credit REAL NOT NULL DEFAULT 0;
ALTER TABLE accounts ADD COLUMN total_debit REAL NOT NULL DEFAULT 0;

-- Backfill once from the source of truth (transactions). New mutations keep
-- these columns updated by delta alongside balance.
UPDATE accounts SET
  total_credit = (SELECT COALESCE(SUM(amount), 0) FROM transactions t WHERE t.account_id = accounts.id AND t.type = 1),
  total_debit  = (SELECT COALESCE(SUM(amount), 0) FROM transactions t WHERE t.account_id = accounts.id AND t.type = 0);
