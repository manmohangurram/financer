package db

import (
	"database/sql"
	"database/sql/driver"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
	sqlite "modernc.org/sqlite"
)

func init() {
	// Register regexp() so rule conditions can be pushed into SQL. Matches the
	// case-insensitive behavior of the rule engine's other string operators.
	sqlite.MustRegisterScalarFunction("regexp", 2, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("regexp expects (pattern, value)")
		}
		pat, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("regexp: pattern must be a string")
		}
		val, ok := args[1].(string)
		if !ok {
			return nil, fmt.Errorf("regexp: value must be a string")
		}
		if !strings.HasPrefix(pat, "(?") {
			pat = "(?i)" + pat
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, err
		}
		return re.MatchString(val), nil
	})
}

//go:embed migrations/*.sql
var migrationsFS embed.FS

func OpenDBs(dbPath string) (writeDB, readDB *sql.DB, err error) {
	if dbPath == "" {
		dbPath = filepath.Join("data", "financer.db")
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("creating data directory: %w", err)
	}

	baseDSN := dbPath + "?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON&_synchronous=NORMAL"

	writeDSN := baseDSN + "&_txlock=immediate"
	writeDB, err = sql.Open("sqlite", writeDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("opening write database: %w", err)
	}
	writeDB.SetMaxOpenConns(1)
	writeDB.SetMaxIdleConns(1)
	writeDB.SetConnMaxLifetime(5 * time.Minute)
	writeDB.SetConnMaxIdleTime(3 * time.Minute)

	if err := writeDB.Ping(); err != nil {
		writeDB.Close()
		return nil, nil, fmt.Errorf("pinging write database: %w", err)
	}

	readDSN := baseDSN + "&_query_only=true"
	readDB, err = sql.Open("sqlite", readDSN)
	if err != nil {
		writeDB.Close()
		return nil, nil, fmt.Errorf("opening read database: %w", err)
	}
	readDB.SetMaxOpenConns(5)
	readDB.SetMaxIdleConns(5)
	readDB.SetConnMaxLifetime(5 * time.Minute)
	readDB.SetConnMaxIdleTime(3 * time.Minute)

	if err := readDB.Ping(); err != nil {
		writeDB.Close()
		readDB.Close()
		return nil, nil, fmt.Errorf("pinging read database: %w", err)
	}

	return writeDB, readDB, nil
}

func RunMigrations(db *sql.DB) error {
	var trackingExists bool
	db.QueryRow(`SELECT name = 'schema_migrations' FROM sqlite_master WHERE type='table' AND name='schema_migrations'`).Scan(&trackingExists)

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("reading migrations: %w", err)
	}

	var upFiles []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		upFiles = append(upFiles, e.Name())
	}
	sort.Strings(upFiles)

	for _, fileName := range upFiles {
		if trackingExists {
			var exists bool
			if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)`, fileName).Scan(&exists); err != nil {
				return fmt.Errorf("checking migration %s: %w", fileName, err)
			}
			if exists {
				continue
			}
		}

		content, err := migrationsFS.ReadFile("migrations/" + fileName)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", fileName, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("beginning transaction for %s: %w", fileName, err)
		}

		statements := splitSQL(string(content))
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("executing migration %s: %w", fileName, err)
			}
		}

		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, fileName); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %s: %w", fileName, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %s: %w", fileName, err)
		}

		slog.Info("applied migration", "file", fileName)
	}

	return nil
}

func splitSQL(content string) []string {
	var statements []string
	var current strings.Builder
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		current.WriteString(line)
		current.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			statements = append(statements, current.String())
			current.Reset()
		}
	}

	if s := strings.TrimSpace(current.String()); s != "" {
		statements = append(statements, s)
	}

	return statements
}
