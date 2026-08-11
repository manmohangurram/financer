-- Restore the Go-era migration tracker (only needed to roll back 000010 on a
-- DB that already ran it; the sqlx migrator ignores this table either way).
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
