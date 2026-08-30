-- Drop the Go-era migration tracker. The sqlx built-in migrator tracks
-- applied versions in `_sqlx_migrations`; `schema_migrations` is dead.
DROP TABLE IF EXISTS schema_migrations;
