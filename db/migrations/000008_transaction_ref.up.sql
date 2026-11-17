ALTER TABLE transactions ADD COLUMN ref_no TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_transactions_user_ref ON transactions(user_id, ref_no) WHERE ref_no != '';
