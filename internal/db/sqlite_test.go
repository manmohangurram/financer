package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	migrate "github.com/mohan9182/financer"
)

func TestMigrationsApply(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "financer.db")
	dbs, err := Open(path)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer dbs.Write.Close()
	defer dbs.Read.Close()

	// Users table should exist after migration.
	var cnt int
	if err := dbs.Write.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&cnt); err != nil {
		t.Fatalf("users table missing: %v", err)
	}
	// schema_migrations may be dropped by migration 000010; our tracker is
	// _financer_applied_migrations, which is independent.
	var ver string
	if err := dbs.Write.QueryRow(`SELECT version FROM _financer_applied_migrations LIMIT 1`).Scan(&ver); err != nil {
		t.Fatalf("migration tracker empty: %v", err)
	}
	if !strings.HasPrefix(ver, "000001_") {
		t.Errorf("first migration = %q", ver)
	}
}

func TestAdoptsSqlxMigrationHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "financer.db")

	// Simulate a database created by the Rust backend: sqlx's history table
	// with every migration marked successful, and no Go tracker.
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE _sqlx_migrations (
		version BIGINT PRIMARY KEY,
		description TEXT NOT NULL,
		installed_on TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		success BOOLEAN NOT NULL,
		checksum BLOB NOT NULL,
		execution_time BIGINT NOT NULL
	)`); err != nil {
		t.Fatal(err)
	}
	entries, err := migrate.MigrationsFS.ReadDir("db/migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		stem := strings.TrimSuffix(e.Name(), ".up.sql") // 000001_users
		parts := strings.SplitN(stem, "_", 2)
		if len(parts) != 2 {
			t.Fatalf("unexpected migration name %q", e.Name())
		}
		var version int64
		if _, err := fmt.Sscanf(parts[0], "%d", &version); err != nil {
			t.Fatal(err)
		}
		if _, err := raw.Exec(
			`INSERT INTO _sqlx_migrations (version, description, success, checksum, execution_time)
			 VALUES (?, ?, 1, ?, 0)`, version, parts[1], []byte{},
		); err != nil {
			t.Fatal(err)
		}
	}
	raw.Close()

	// Opening must adopt the history instead of re-running the migrations (the
	// ALTER TABLE steps would fail on an already-migrated schema).
	dbs, err := Open(path)
	if err != nil {
		t.Fatalf("opening an sqlx-migrated database: %v", err)
	}
	defer dbs.Write.Close()
	defer dbs.Read.Close()

	var count int
	if err := dbs.Write.QueryRow(`SELECT COUNT(*) FROM _financer_applied_migrations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("sqlx history was not adopted")
	}
}

func TestWALModeActive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "financer.db")
	dbs, err := Open(path)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer dbs.Write.Close()
	defer dbs.Read.Close()

	var mode string
	if err := dbs.Write.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("reading journal mode: %v", err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Fatalf("journal mode = %q, want wal", mode)
	}
}

func TestOpenCreatesDataDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")
	path := filepath.Join(dir, "financer.db")
	dbs, err := Open(path)
	if err != nil {
		t.Fatalf("opening db in new dir: %v", err)
	}
	defer dbs.Write.Close()
	defer dbs.Read.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("db file not created: %v", err)
	}
}
