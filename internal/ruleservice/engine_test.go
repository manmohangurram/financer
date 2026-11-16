package ruleservice

import (
	"testing"

	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
)

func matchData() *TransactionMatchData {
	return &TransactionMatchData{
		Name:       "Netflix",
		Amount:     15.99,
		Type:       string(model.TransactionTypeDebit),
		AccountID:  "acc-1",
		Categories: []string{"Entertainment"},
	}
}

func TestApplyNameOp(t *testing.T) {
	cases := []struct{ cur, val, op, want string }{
		{"Netflix", "Fee", string(model.ActionOpRename), "Fee"},
		{"Netflix", "Fee", string(model.ActionOpAddPrefix), "FeeNetflix"},
		{"Netflix", "-Sub", string(model.ActionOpAddSuffix), "Netflix-Sub"},
		{"Netflix", "", string(model.ActionOpAddSuffix), "Netflix"},
	}
	for _, c := range cases {
		if got := applyNameOp(c.cur, c.val, c.op); got != c.want {
			t.Errorf("applyNameOp(%q,%q,%q) = %q, want %q", c.cur, c.val, c.op, got, c.want)
		}
	}
}

func cond(field, op, pattern string) repo.RuleConditionRow {
	return repo.RuleConditionRow{MatchField: field, Operator: op, Pattern: pattern}
}

func TestMatchRuleData(t *testing.T) {
	data := matchData()
	cases := []struct {
		name  string
		logic string
		conds []repo.RuleConditionRow
		want  bool
	}{
		{
			name:  "or contains",
			logic: string(model.RuleLogicOr),
			conds: []repo.RuleConditionRow{cond(string(model.MatchFieldName), string(model.MatchOperatorContains), "netflix")},
			want:  true,
		},
		{
			name:  "or amount greater than no match",
			logic: string(model.RuleLogicOr),
			conds: []repo.RuleConditionRow{cond(string(model.MatchFieldAmount), string(model.MatchOperatorGreaterThan), "100")},
			want:  false,
		},
		{
			name:  "regex case insensitive",
			logic: string(model.RuleLogicOr),
			conds: []repo.RuleConditionRow{cond(string(model.MatchFieldName), string(model.MatchOperatorRegex), "NETFLIX")},
			want:  true,
		},
		{
			name:  "and category equals plus name contains",
			logic: string(model.RuleLogicAnd),
			conds: []repo.RuleConditionRow{
				cond(string(model.MatchFieldCategory), string(model.MatchOperatorEquals), "Entertainment"),
				cond(string(model.MatchFieldName), string(model.MatchOperatorContains), "netflix"),
			},
			want: true,
		},
		{
			name:  "and one fails",
			logic: string(model.RuleLogicAnd),
			conds: []repo.RuleConditionRow{
				cond(string(model.MatchFieldCategory), string(model.MatchOperatorEquals), "Entertainment"),
				cond(string(model.MatchFieldName), string(model.MatchOperatorContains), "spotify"),
			},
			want: false,
		},
		{
			name:  "empty conditions never match",
			logic: string(model.RuleLogicOr),
			conds: nil,
			want:  false,
		},
		{
			name:  "starts with",
			logic: string(model.RuleLogicOr),
			conds: []repo.RuleConditionRow{cond(string(model.MatchFieldName), string(model.MatchOperatorStartsWith), "Net")},
			want:  true,
		},
		{
			name:  "ends with",
			logic: string(model.RuleLogicOr),
			conds: []repo.RuleConditionRow{cond(string(model.MatchFieldName), string(model.MatchOperatorEndsWith), "flix")},
			want:  true,
		},
		{
			name:  "less than",
			logic: string(model.RuleLogicOr),
			conds: []repo.RuleConditionRow{cond(string(model.MatchFieldAmount), string(model.MatchOperatorLessThan), "20")},
			want:  true,
		},
		{
			name:  "type equals",
			logic: string(model.RuleLogicOr),
			conds: []repo.RuleConditionRow{cond(string(model.MatchFieldType), string(model.MatchOperatorEquals), "DEBIT")},
			want:  true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MatchRuleData(data, c.logic, c.conds); got != c.want {
				t.Errorf("MatchRuleData = %v, want %v", got, c.want)
			}
		})
	}
}

func TestApplyRuleActionsInOrder(t *testing.T) {
	view := &repo.TransactionView{Name: "Netflix", Amount: 15.99, Type: "DEBIT", AccountID: "acc-1"}
	ApplyRuleActions(view, []repo.RuleActionRow{
		{ActionType: string(model.ActionTypeSetName), NameOp: string(model.ActionOpRename), Value: "Netflix Subscription"},
		{ActionType: string(model.ActionTypeSetCategory), CategoryID: "cat-1"},
		{ActionType: string(model.ActionTypeSetTransferAccount), TransferAccountID: "acc-2"},
	})
	if view.Name != "Netflix Subscription" {
		t.Errorf("name = %q", view.Name)
	}
	if len(view.CategoryIDs) != 1 || view.CategoryIDs[0] != "cat-1" {
		t.Errorf("categories = %v", view.CategoryIDs)
	}
}

func TestFormatAmount(t *testing.T) {
	cases := map[float64]string{15.99: "15.99", 100: "100", 0.5: "0.5"}
	for in, want := range cases {
		if got := FormatAmount(in); got != want {
			t.Errorf("FormatAmount(%v) = %q, want %q", in, got, want)
		}
	}
}
