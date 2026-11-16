// Package db opens the SQLite write + read pools and runs migrations.
// Uses glebarez/sqlite (pure Go, no CGO). Storage tuning mirrors the Rust
// backend: WAL journal, single-writer write pool + a read pool, foreign keys ON.
package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/glebarez/sqlite"
	"github.com/mohan9182/financer"
)

type DBs struct {
	Write *sql.DB
	Read  *sql.DB
}

func Open(dbPath string) (*DBs, error) {
	if dbPath == "" {
		dbPath = filepath.Join("data", "financer.db")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating data directory: %w", err)
	}

	// glebarez/sqlite DSN query params mirror the sqlx options.
	base := dbPath + "?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON&_synchronous=NORMAL&_txlock=immediate"

	write, err := sql.Open("sqlite", base)
	if err != nil {
		return nil, fmt.Errorf("opening write database: %w", err)
	}
	write.SetMaxOpenConns(1)
	write.SetMaxIdleConns(1)
	write.SetConnMaxLifetime(5 * time.Minute)
	write.SetConnMaxIdleTime(3 * time.Minute)
	if err := write.Ping(); err != nil {
		write.Close()
		return nil, fmt.Errorf("pinging write database: %w", err)
	}
	// Ensure WAL is active (database-level pragma, persists across connections).
	if _, err := write.Exec("PRAGMA journal_mode=WAL"); err != nil {
		write.Close()
		return nil, fmt.Errorf("setting WAL journal mode: %w", err)
	}

	read, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON&_synchronous=NORMAL&_query_only=true&mode=ro")
	if err != nil {
		write.Close()
		return nil, fmt.Errorf("opening read database: %w", err)
	}
	read.SetMaxOpenConns(5)
	read.SetMaxIdleConns(5)
	read.SetConnMaxLifetime(5 * time.Minute)
	read.SetConnMaxIdleTime(3 * time.Minute)
	if err := read.Ping(); err != nil {
		write.Close()
		read.Close()
		return nil, fmt.Errorf("pinging read database: %w", err)
	}

	if err := RunMigrations(write); err != nil {
		write.Close()
		read.Close()
		return nil, err
	}

	return &DBs{Write: write, Read: read}, nil
}

func RunMigrations(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS _financer_applied_migrations (
		version TEXT PRIMARY KEY
	)`); err != nil {
		return fmt.Errorf("creating migration tracker: %w", err)
	}

	entries, err := migrate.MigrationsFS.ReadDir("db/migrations")
	if err != nil {
		return fmt.Errorf("reading migrations: %w", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		files = append(files, "db/migrations/"+e.Name())
	}
	sort.Strings(files)

	// Adopt an existing sqlx migration history. Production databases created by
	// the Rust backend already have every migration applied; re-running the
	// ALTER TABLE steps would fail. When the Go tracker is empty but
	// `_sqlx_migrations` exists, seed it from the successful sqlx rows.
	if err := adoptSqlxMigrations(db, files); err != nil {
		return err
	}

	for _, f := range files {
		base := filepath.Base(f)
		var exists bool
		if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM _financer_applied_migrations WHERE version = ?)`, base).Scan(&exists); err != nil {
			return fmt.Errorf("checking migration %s: %w", f, err)
		}
		if exists {
			continue
		}
		content, err := migrate.MigrationsFS.ReadFile(f)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", f, err)
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("beginning transaction for %s: %w", f, err)
		}
		for _, stmt := range splitSQL(string(content)) {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("executing migration %s: %w", f, err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO _financer_applied_migrations (version) VALUES (?)`, base); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %s: %w", f, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %s: %w", f, err)
		}
	}
	return nil
}

// adoptSqlxMigrations copies a successful sqlx history into the Go tracker so a
// database migrated by the Rust backend is not migrated again.
func adoptSqlxMigrations(db *sql.DB, files []string) error {
	var tracked int
	if err := db.QueryRow(`SELECT COUNT(*) FROM _financer_applied_migrations`).Scan(&tracked); err != nil {
		return fmt.Errorf("counting applied migrations: %w", err)
	}
	if tracked > 0 {
		return nil
	}
	var hasSqlx bool
	if err := db.QueryRow(
		`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = '_sqlx_migrations')`,
	).Scan(&hasSqlx); err != nil {
		return fmt.Errorf("checking sqlx history: %w", err)
	}
	if !hasSqlx {
		return nil
	}

	rows, err := db.Query(`SELECT version, description FROM _sqlx_migrations WHERE success = 1`)
	if err != nil {
		return fmt.Errorf("reading sqlx history: %w", err)
	}
	known := make(map[string]bool, len(files))
	for _, f := range files {
		known[filepath.Base(f)] = true
	}
	// Buffer before inserting: the write pool has one connection, so an Exec
	// while the rows iterator holds it would deadlock.
	var adopted []string
	for rows.Next() {
		var version int64
		var description string
		if err := rows.Scan(&version, &description); err != nil {
			rows.Close()
			return fmt.Errorf("scanning sqlx history: %w", err)
		}
		name := fmt.Sprintf("%06d_%s.up.sql", version, description)
		// Only adopt migrations this binary knows; anything else was applied by
		// a newer backend and is left alone.
		if known[name] {
			adopted = append(adopted, name)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("reading sqlx history: %w", err)
	}
	rows.Close()

	for _, name := range adopted {
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO _financer_applied_migrations (version) VALUES (?)`, name,
		); err != nil {
			return fmt.Errorf("adopting migration %s: %w", name, err)
		}
	}
	if len(adopted) > 0 {
		slog.Info("adopted sqlx migration history", "migrations", len(adopted))
	}
	return nil
}

func splitSQL(content string) []string {
	var stmts []string
	var cur strings.Builder
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		cur.WriteString(line)
		cur.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			if s := strings.TrimSpace(cur.String()); s != "" {
				stmts = append(stmts, s)
			}
			cur.Reset()
		}
	}
	if s := strings.TrimSpace(cur.String()); s != "" {
		stmts = append(stmts, s)
	}
	return stmts
}
