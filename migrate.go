// Package migrate embeds the SQLite migration SQL files at compile time so the
// binary is self-contained (matches the Rust backend's sqlx embed). The files
// live in db/migrations/ and are the single source of truth shared with Rust.
package migrate

import "embed"

//go:embed db/migrations/*.up.sql
var MigrationsFS embed.FS
