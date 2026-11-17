DROP INDEX IF EXISTS idx_transactions_user_ref;

ALTER TABLE transactions DROP COLUMN ref_no;
