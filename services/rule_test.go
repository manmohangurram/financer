package services

import (
	"testing"

	"github.com/mohan9182/financer/api"
	"github.com/mohan9182/financer/repository"
)

func TestApplyNameOp(t *testing.T) {
	cases := []struct {
		name, val string
		op        api.RuleActionOp
		want      string
	}{
		{"rename", "Fee", api.RuleActionOp_RULE_ACTION_OP_RENAME, "Fee"},
		{"default", "Fee", api.RuleActionOp_RULE_ACTION_OP_UNSPECIFIED, "Fee"},
		{"prefix", "Fee", api.RuleActionOp_RULE_ACTION_OP_ADD_PREFIX, "FeeNetflix"},
		{"prefix-no-space", "FEE ", api.RuleActionOp_RULE_ACTION_OP_ADD_PREFIX, "FEE Netflix"},
		{"suffix", "-Sub", api.RuleActionOp_RULE_ACTION_OP_ADD_SUFFIX, "Netflix-Sub"},
		{"suffix-empty", "", api.RuleActionOp_RULE_ACTION_OP_ADD_SUFFIX, "Netflix"},
	}
	for _, tc := range cases {
		if got := applyNameOp("Netflix", tc.val, tc.op); got != tc.want {
			t.Errorf("%s: applyNameOp(%q, %q, %v) = %q, want %q", tc.name, "Netflix", tc.val, tc.op, got, tc.want)
		}
	}
}

func TestMatchRuleData(t *testing.T) {
	txn := txnForMatchData{Name: "Netflix", Amount: 15.99, Type: 0, Categories: []string{"Entertainment"}}

	cases := []struct {
		logic string
		conds []repository.RuleCondData
		want  bool
	}{
		{
			logic: "OR",
			conds: []repository.RuleCondData{{MatchField: "RULE_MATCH_FIELD_NAME", Operator: "RULE_MATCH_OPERATOR_CONTAINS", Pattern: "netflix"}},
			want:  true,
		},
		{
			logic: "OR",
			conds: []repository.RuleCondData{{MatchField: "RULE_MATCH_FIELD_AMOUNT", Operator: "RULE_MATCH_OPERATOR_GREATER_THAN", Pattern: "100"}},
			want:  false,
		},
		{
			logic: "OR",
			conds: []repository.RuleCondData{{MatchField: "RULE_MATCH_FIELD_NAME", Operator: "RULE_MATCH_OPERATOR_REGEX", Pattern: "NETFLIX"}},
			want:  true,
		},
		{
			logic: "AND",
			conds: []repository.RuleCondData{
				{MatchField: "RULE_MATCH_FIELD_CATEGORY", Operator: "RULE_MATCH_OPERATOR_EQUALS", Pattern: "Entertainment"},
				{MatchField: "RULE_MATCH_FIELD_NAME", Operator: "RULE_MATCH_OPERATOR_CONTAINS", Pattern: "netflix"},
			},
			want: true,
		},
	}

	for _, tc := range cases {
		if got := matchRuleData(txn, tc.logic, tc.conds); got != tc.want {
			t.Errorf("logic=%s want %v got %v", tc.logic, tc.want, got)
		}
	}
}

func TestApplyRuleActionsOrder(t *testing.T) {
	txn := &api.TransactionResponse{Name: "Netflix"}
	applyRuleActions(txn, []*api.RuleAction{
		{SetName: "Netflix Subscription", SetNameOp: api.RuleActionOp_RULE_ACTION_OP_RENAME},
		{SetCategoryId: "cat-1"},
	})
	if txn.Name != "Netflix Subscription" || len(txn.CategoryIds) != 1 || txn.CategoryIds[0] != "cat-1" {
		t.Fatalf("expected rename+category, got name=%q cats=%v", txn.Name, txn.CategoryIds)
	}
}
