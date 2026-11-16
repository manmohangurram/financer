package ruleservice

import (
	"path/filepath"
	"testing"

	"github.com/mohan9182/financer/internal/db"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
	"github.com/mohan9182/financer/internal/repo/sqlite"
	"github.com/mohan9182/financer/internal/transactionservice"
	"github.com/mohan9182/financer/internal/transferservice"
)

type testEnv struct {
	rules      *Service
	transfer   *TransferRuleService
	txns       *transactionservice.Service
	txnRepo    repo.TransactionRepo
	ruleRepo   repo.RuleRepo
	uid        string
	accountA   string
	accountB   string
	categoryID string
	cleanup    func()
}

func setup(t *testing.T) *testEnv {
	t.Helper()
	dbs, err := db.Open(filepath.Join(t.TempDir(), "financer.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	users := sqlite.NewUserRepo(dbs.Write)
	accounts := sqlite.NewAccountRepo(dbs.Write)
	categories := sqlite.NewCategoryRepo(dbs.Write)
	txns := sqlite.NewTransactionRepo(dbs.Write)
	rules := sqlite.NewRuleRepo(dbs.Write)

	uid, err := users.Create("r@test.com", "hash", "Rules")
	if err != nil {
		t.Fatal(err)
	}
	a, err := accounts.Create(uid, "HDFC", "Main", "SAVINGS", "1111")
	if err != nil {
		t.Fatal(err)
	}
	b, err := accounts.Create(uid, "ICICI", "Savings", "SAVINGS", "2222")
	if err != nil {
		t.Fatal(err)
	}
	cats, err := categories.Create(uid, []repo.CategoryCreateInput{{Name: "Entertainment"}})
	if err != nil {
		t.Fatal(err)
	}

	ruleSvc := New(rules, categories, txns)
	transferSvc := transferservice.New(txns, accounts)
	transferRuleSvc := NewTransferRuleService(rules, txns, accounts, transferSvc)
	txnSvc := transactionservice.New(txns, accounts, categories).WithRule(ruleSvc).WithTransferRule(transferRuleSvc)

	return &testEnv{
		rules: ruleSvc, transfer: transferRuleSvc, txns: txnSvc,
		txnRepo: txns, ruleRepo: rules,
		uid: uid, accountA: a.ID, accountB: b.ID, categoryID: cats[0].ID,
		cleanup: func() {
			dbs.Write.Close()
			dbs.Read.Close()
		},
	}
}

func ptr[T any](v T) *T { return &v }

func containsName(pattern string) []model.RuleCondition {
	return []model.RuleCondition{{
		MatchField: model.MatchFieldName,
		Operator:   model.MatchOperatorContains,
		Pattern:    pattern,
	}}
}

func TestRuleCrudAndOverlay(t *testing.T) {
	env := setup(t)
	defer env.cleanup()

	rule, err := env.rules.Create(env.uid, "Netflix", 10, string(model.RuleLogicOr),
		containsName("netflix"),
		[]model.RuleAction{{SetName: "Netflix Subscription"}, {SetCategoryID: env.categoryID}})
	if err != nil {
		t.Fatal(err)
	}
	if rule.ID == "" || rule.CreatedAt == "" {
		t.Fatalf("rule = %+v", rule)
	}
	if len(rule.Conditions) != 1 || rule.Conditions[0].MatchField != model.MatchFieldName {
		t.Fatalf("conditions = %+v", rule.Conditions)
	}
	if len(rule.Actions) != 2 || rule.Actions[1].SetCategoryID != env.categoryID {
		t.Fatalf("actions = %+v", rule.Actions)
	}

	// Empty name / no conditions are rejected.
	if _, err := env.rules.Create(env.uid, "", 0, "OR", containsName("x"), nil); err == nil {
		t.Fatal("expected error for empty name")
	}
	if _, err := env.rules.Create(env.uid, "x", 0, "OR", nil, nil); err == nil {
		t.Fatal("expected error for no conditions")
	}

	// A transaction created after the rule is snapshotted with clean_name and
	// the rule's category; the raw name is preserved.
	out, err := env.txns.Create(env.uid, []transactionservice.Request{{
		ID: "txn-1", Name: "NETFLIX.COM", Amount: 15.99, Type: "DEBIT",
		OccurredAt: "2026-11-01 10:00:00 +0000 UTC", AccountID: env.accountA,
	}})
	if err != nil || !out.Success {
		t.Fatalf("txn create: %+v err=%v", out, err)
	}
	rows, err := env.txnRepo.List(env.uid, &repo.TransactionListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	row := rows.Rows[0]
	if row.Txn.Name != "NETFLIX.COM" {
		t.Fatalf("raw name overwritten: %q", row.Txn.Name)
	}
	if row.Txn.CleanName == nil || *row.Txn.CleanName != "Netflix Subscription" {
		t.Fatalf("clean name = %v", row.Txn.CleanName)
	}
	if len(row.CategoryIDs) != 1 || row.CategoryIDs[0] != env.categoryID {
		t.Fatalf("categories = %v", row.CategoryIDs)
	}

	// Update replaces conditions/actions and is refetchable.
	updated, err := env.rules.Update(env.uid, rule.ID, "Netflix renamed", 20, string(model.RuleLogicAnd),
		[]model.RuleCondition{{MatchField: model.MatchFieldAmount, Operator: model.MatchOperatorGreaterThan, Pattern: "10"}},
		[]model.RuleAction{{SetNameOp: ptr(model.ActionOpAddPrefix), SetName: "SUB: "}})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Netflix renamed" || updated.Logic != model.RuleLogicAnd {
		t.Fatalf("updated = %+v", updated)
	}
	if len(updated.Actions) != 1 || updated.Actions[0].SetNameOp == nil || *updated.Actions[0].SetNameOp != model.ActionOpAddPrefix {
		t.Fatalf("updated actions = %+v", updated.Actions)
	}
	if _, err := env.rules.Update(env.uid, "missing", "x", 0, "OR", containsName("x"), nil); err == nil {
		t.Fatal("expected not-found on update")
	}

	list, err := env.rules.List(env.uid)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list = %d", len(list))
	}

	// Preview matches existing rows (raw name).
	views, err := env.rules.Preview(env.uid, string(model.RuleLogicOr), containsName("netflix"), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || views[0].Name != "NETFLIX.COM" {
		t.Fatalf("preview = %+v", views)
	}
	if _, err := env.rules.Preview(env.uid, "OR", []model.RuleCondition{{
		MatchField: model.MatchFieldName, Operator: model.MatchOperatorRegex, Pattern: "(",
	}}, 5); err == nil {
		t.Fatal("expected error for invalid regex")
	}

	if err := env.rules.Delete(env.uid, rule.ID); err != nil {
		t.Fatal(err)
	}
	if err := env.rules.Delete(env.uid, rule.ID); err == nil {
		t.Fatal("expected not-found on second delete")
	}
}

func TestApplyToExistingAndRun(t *testing.T) {
	env := setup(t)
	defer env.cleanup()

	// Seed a transaction before any rule exists.
	if _, err := env.txns.Create(env.uid, []transactionservice.Request{{
		ID: "txn-1", Name: "Spotify", Amount: 9.99, Type: "DEBIT",
		OccurredAt: "2026-11-01 10:00:00 +0000 UTC", AccountID: env.accountA,
	}}); err != nil {
		t.Fatal(err)
	}
	rule, err := env.rules.Create(env.uid, "Spotify", 0, "OR", containsName("spotify"),
		[]model.RuleAction{{SetName: "Spotify Premium"}})
	if err != nil {
		t.Fatal(err)
	}

	changed, err := env.rules.ApplyToExisting(env.uid)
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Fatalf("changed = %d", changed)
	}
	rows, err := env.txnRepo.List(env.uid, &repo.TransactionListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if rows.Rows[0].Txn.CleanName == nil || *rows.Rows[0].Txn.CleanName != "Spotify Premium" {
		t.Fatalf("clean name = %v", rows.Rows[0].Txn.CleanName)
	}

	n, err := env.rules.Run(env.uid, rule.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("run changed = %d", n)
	}
	if _, err := env.rules.Run(env.uid, "missing"); err == nil {
		t.Fatal("expected not-found on run")
	}
}

func TestTransferRuleAutoLinksOnCreate(t *testing.T) {
	env := setup(t)
	defer env.cleanup()

	// Rule: any name containing "moved" transfers to account B.
	rule, err := env.ruleRepo.Create(env.uid, "Move to savings", 0, "OR",
		[]repo.RuleConditionRow{{
			MatchField: string(model.MatchFieldName),
			Operator:   string(model.MatchOperatorContains),
			Pattern:    "moved",
		}},
		[]repo.RuleActionRow{{
			ActionType:        string(model.ActionTypeSetTransferAccount),
			TransferAccountID: env.accountB,
		}})
	if err != nil {
		t.Fatal(err)
	}

	out, err := env.txns.Create(env.uid, []transactionservice.Request{{
		ID: "debit-1", Name: "Moved to savings", Amount: 500, Type: "DEBIT",
		OccurredAt: "2026-11-02 09:00:00 +0000 UTC", AccountID: env.accountA,
	}})
	if err != nil || !out.Success {
		t.Fatalf("create: %+v err=%v", out, err)
	}

	rows, err := env.txnRepo.List(env.uid, &repo.TransactionListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if rows.TotalCount != 2 {
		t.Fatalf("expected a created counterpart, got %d rows", rows.TotalCount)
	}
	for _, row := range rows.Rows {
		if row.LinkID == "" {
			t.Fatalf("row %s not linked", row.Txn.Name)
		}
	}

	// RunRule on the same rule: the already-linked debit is skipped, so add a
	// second unlinked debit that matches.
	if _, err := env.txnRepo.Create(env.uid, []repo.CreateTransactionInput{{Txn: &repo.TransactionRow{
		ID: "debit-2", Name: "Moved again", Amount: 25, Type: "DEBIT",
		OccurredAt: "2026-11-03 09:00:00 +0000 UTC", AccountID: env.accountA,
		CreatedAt: "2026-11-03 09:00:00 +0000 UTC",
	}}}); err != nil {
		t.Fatal(err)
	}
	run, err := env.transfer.RunRule(env.uid, rule.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Matched != 1 || run.Linked != 1 || run.Created != 1 {
		t.Fatalf("run = %+v", run)
	}
}
