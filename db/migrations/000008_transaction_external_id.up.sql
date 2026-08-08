-- Stable external key for idempotent imports: a duplicate (user_id, external_id)
-- is skipped on retry instead of creating a second transaction.
ALTER TABLE transactions ADD COLUMN external_id TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_external_id ON transactions(user_id, external_id);
