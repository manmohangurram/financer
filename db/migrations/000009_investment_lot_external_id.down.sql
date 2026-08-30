DROP INDEX IF EXISTS idx_lots_user_external;
ALTER TABLE investment_lots DROP COLUMN external_id;
