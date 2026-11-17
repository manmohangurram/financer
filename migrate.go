// Package migrate embeds the SQLite schema at compile time so the binary is
// self-contained. db/migrations/ holds the single baseline schema; future
// changes are added as numbered *.up.sql / *.down.sql pairs and applied in
// order by internal/db.
package migrate

import "embed"

//go:embed db/migrations/*.up.sql
var MigrationsFS embed.FS
