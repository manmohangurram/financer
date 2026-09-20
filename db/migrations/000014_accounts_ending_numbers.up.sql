-- Trailing digits of the account/card number, used to match bank alert emails
-- to an account. Nullable for rows created before this migration; required by
-- the service on create.
ALTER TABLE accounts ADD COLUMN ending_numbers TEXT;

CREATE INDEX IF NOT EXISTS idx_accounts_user_ending ON accounts(user_id, ending_numbers);
