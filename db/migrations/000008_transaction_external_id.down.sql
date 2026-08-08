DROP INDEX IF EXISTS idx_transactions_external_id;
ALTER TABLE transactions DROP COLUMN external_id;
