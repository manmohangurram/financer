-- Rule-resolved display name (nullable); `name` stays the raw original.
ALTER TABLE transactions ADD COLUMN clean_name TEXT;
