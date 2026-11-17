package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	var cnt int
	if err := dbs.Write.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&cnt); err != nil {
		t.Fatalf("users table missing: %v", err)
	}

	var versions []string
	rows, err := dbs.Write.Query(`SELECT version FROM _financer_applied_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		versions = append(versions, v)
	}
	if len(versions) == 0 || !strings.HasPrefix(versions[0], "000001_initial_schema") {
		t.Fatalf("applied migrations = %v, want the baseline first", versions)
	}
	if versions[len(versions)-1] != "000003_mail_body.up.sql" {
		t.Fatalf("last migration = %q", versions[len(versions)-1])
	}

	// The baseline must carry the final columns; a compile/plan check is enough.
	for _, q := range []string{
		`SELECT avatar_url, token_version FROM users`,
		`SELECT total_credit, total_debit, ending_numbers FROM accounts`,
		`SELECT clean_name, external_id FROM transactions`,
		`SELECT scope FROM user_keys`,
		`SELECT parsed_json, transaction_id FROM mail_messages`,
		`SELECT body_text, from_name, template_id FROM mail_messages`,
	} {
		r, err := dbs.Write.Query(q)
		if err != nil {
			t.Errorf("%s: %v", q, err)
			continue
		}
		r.Close()
	}

	for _, idx := range []string{
		"idx_transactions_user_occurred",
		"idx_transactions_user_account_occurred",
		"idx_transactions_external_id",
		"idx_transaction_categories_category",
		"idx_mail_messages_user_created",
		"idx_rules_user_priority",
		"idx_user_keys_key_hash",
		"idx_transfer_links_debit",
		"idx_rule_conditions_rule",
		"idx_rule_actions_rule",
	} {
		var exists bool
		if err := dbs.Write.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'index' AND name = ?)`, idx,
		).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Errorf("missing index %s", idx)
		}
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
