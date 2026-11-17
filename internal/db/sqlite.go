// Package db opens the SQLite write + read pools and applies the embedded
// schema. Uses glebarez/sqlite (pure Go, no CGO). Storage tuning mirrors the
// previous backend: WAL journal, single-writer write pool + a read pool,
// foreign keys ON.
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

	slog.Info("database ready", "path", dbPath, "journal_mode", "wal", "write_pool", 1, "read_pool", 5)
	return &DBs{Write: write, Read: read}, nil
}

// RunMigrations applies every embedded db/migrations/*.up.sql file that is not
// yet recorded in _financer_applied_migrations. The schema is a single baseline
// migration; there is no legacy history to reconcile.
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

	applied := 0
	for _, f := range files {
		base := filepath.Base(f)
		var exists bool
		if err := db.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM _financer_applied_migrations WHERE version = ?)`, base,
		).Scan(&exists); err != nil {
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
		applied++
		slog.Debug("migration applied", "migration", base)
	}
	if applied > 0 {
		slog.Info("database migrations complete", "applied", applied)
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
