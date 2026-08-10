ALTER TABLE investment_lots ADD COLUMN external_id TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_lots_user_external ON investment_lots(user_id, external_id) WHERE external_id IS NOT NULL;
