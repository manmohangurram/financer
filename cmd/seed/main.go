package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/mohan9182/financer/db"
	"golang.org/x/crypto/bcrypt"
)
 
func main() {
	// Match main.go's data layout: FINANCER_DATA_DIR/db/financer.db.
	dataDir := os.Getenv("FINANCER_DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	dbPath := os.Getenv("FINANCER_DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join(dataDir, "db", "financer.db")
	}
	writeDB, _, err := db.OpenDBs(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer writeDB.Close()
	if err := db.RunMigrations(writeDB); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	ctx := context.Background()

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	userID := uuid.New().String()
	now := time.Now().UTC()
	_, err = writeDB.ExecContext(ctx,
		`INSERT OR IGNORE INTO users (id, email, password_hash, name, created_at) VALUES (?, ?, ?, ?, ?)`,
		userID, "demo@financer.app", string(hashedPassword), "Demo User", now,
	)
	if err != nil {
		log.Printf("User may already exist: %v", err)
	} else {
		fmt.Printf("Created demo user: demo@financer.app / password123 (ID: %s)\n", userID)
	}

	accounts := []struct {
		bankName    string
		nickname    string
		amount      float32
		accountType int
	}{
		{"HDFC", "Main Checking", 542050.00, 1},        // ACCOUNT_TYPE_CHECKING
		{"ICICI", "Emergency Fund", 1230000.00, 2},     // ACCOUNT_TYPE_SAVINGS
		{"SBI", "Credit Card", -85025.00, 3},           // ACCOUNT_TYPE_CREDIT_CARD
	}

	accountIDs := make([]string, len(accounts))
	for i, a := range accounts {
		id := uuid.New().String()
		accountIDs[i] = id
		_, err := writeDB.ExecContext(ctx,
			`INSERT OR IGNORE INTO accounts (id, user_id, bank_name, balance, nickname, type, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			id, userID, a.bankName, a.amount, a.nickname, a.accountType, now,
		)
		if err != nil {
			log.Printf("Account insert error: %v", err)
			continue
		}
		fmt.Printf("Created account: %s (%s)\n", a.nickname, id)
	}

	categories := []string{"Food & Dining", "Transportation", "Housing", "Entertainment", "Shopping", "Income", "Utilities", "Healthcare"}
	categoryIDs := make([]string, len(categories))
	for i, name := range categories {
		id := uuid.New().String()
		categoryIDs[i] = id
		_, err := writeDB.ExecContext(ctx,
			`INSERT OR IGNORE INTO categories (id, name, created_at, user_id) VALUES (?, ?, ?, ?)`,
			id, name, now, userID,
		)
		if err != nil {
			log.Printf("Category insert error: %v", err)
			continue
		}
		fmt.Printf("Created category: %s\n", name)
	}

	transactions := []struct {
		name      string
		amount    float32
		txType    int
		daysAgo   int
		accountID string
		catIndex  int
	}{
		{"Grocery Store", 8550.00, 0, 1, accountIDs[0], 0},
		{"Gas Station", 4500.00, 0, 2, accountIDs[0], 1},
		{"Rent Payment", 150000.00, 0, 5, accountIDs[0], 2},
		{"Netflix", 1599.00, 0, 3, accountIDs[0], 3},
		{"Amazon Purchase", 12999.00, 0, 4, accountIDs[0], 4},
		{"Salary Deposit", 450000.00, 1, 5, accountIDs[0], 5},
		{"Electric Bill", 9500.00, 0, 6, accountIDs[0], 6},
		{"Doctor Visit", 15000.00, 0, 7, accountIDs[0], 7},
		{"Coffee Shop", 1250.00, 0, 1, accountIDs[0], 0},
		{"Uber Ride", 2200.00, 0, 2, accountIDs[0], 1},
		{"Freelance Payment", 80000.00, 1, 3, accountIDs[0], 5},
		{"Internet Bill", 7999.00, 0, 4, accountIDs[0], 6},
		{"Pharmacy", 3500.00, 0, 5, accountIDs[0], 7},
		{"Movie Tickets", 3200.00, 0, 6, accountIDs[0], 3},
		{"Transfer to Savings", 50000.00, 0, 7, accountIDs[0], 0},
		{"Transfer from Checking", 50000.00, 1, 7, accountIDs[1], 0},
	}

	for _, t := range transactions {
		id := uuid.New().String()
		occurredAt := now.Add(-time.Duration(t.daysAgo) * 24 * time.Hour)
		_, err := writeDB.ExecContext(ctx,
			`INSERT OR IGNORE INTO transactions (id, user_id, account_id, name, amount, type, occurred_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, userID, t.accountID, t.name, t.amount, t.txType, occurredAt, now,
		)
		if err != nil {
			log.Printf("Transaction insert error: %v", err)
			continue
		}
		if t.catIndex < len(categoryIDs) {
			writeDB.ExecContext(ctx,
				`INSERT OR IGNORE INTO transaction_categories (transaction_id, category_id) VALUES (?, ?)`,
				id, categoryIDs[t.catIndex],
			)
		}
		fmt.Printf("Created transaction: %s (₹%.2f)\n", t.name, t.amount)
	}

	// Keep the cached account credit/debit totals consistent with the seeded
	// transactions (the dashboard reads these; they must match).
	if _, err := writeDB.ExecContext(ctx,
		`UPDATE accounts SET
			total_credit = (SELECT COALESCE(SUM(amount), 0) FROM transactions t WHERE t.account_id = accounts.id AND t.type = 1),
			total_debit  = (SELECT COALESCE(SUM(amount), 0) FROM transactions t WHERE t.account_id = accounts.id AND t.type = 0)
		 WHERE user_id = ?`, userID,
	); err != nil {
		log.Printf("Account totals backfill error: %v", err)
	}

	rules := []struct {
		name     string
		priority int32
		logic    string
		field    string
		operator string
		pattern  string
	}{
		{"Food Purchases", 10, "OR", "RULE_MATCH_FIELD_CATEGORY", "RULE_MATCH_OPERATOR_EQUALS", "Food & Dining"},
		{"Large Transactions", 5, "OR", "RULE_MATCH_FIELD_AMOUNT", "RULE_MATCH_OPERATOR_GREATER_THAN", "100"},
		{"Income", 8, "OR", "RULE_MATCH_FIELD_TYPE", "RULE_MATCH_OPERATOR_EQUALS", "1"},
	}

	for _, a := range rules {
		ruleID := uuid.New().String()
		_, err := writeDB.ExecContext(ctx,
			`INSERT OR IGNORE INTO rules (id, name, priority, logic, created_at, user_id) VALUES (?, ?, ?, ?, ?, ?)`,
			ruleID, a.name, a.priority, a.logic, now, userID,
		)
		if err != nil {
			log.Printf("Rule insert error: %v", err)
			continue
		}
		condID := uuid.New().String()
		writeDB.ExecContext(ctx,
			`INSERT OR IGNORE INTO rule_conditions (id, rule_id, match_field, operator, pattern) VALUES (?, ?, ?, ?, ?)`,
			condID, ruleID, a.field, a.operator, a.pattern,
		)
		fmt.Printf("Created rule: %s\n", a.name)
	}

	fmt.Println("\nSeed data complete!")
	fmt.Println("Login with: demo@financer.app / password123")
}