package repository

import (
	"strings"
	"testing"
)

func TestBuildRuleQueryNameContains(t *testing.T) {
	query, args, err := buildRuleQuery("u1", "OR", []RuleCondData{{
		MatchField: "RULE_MATCH_FIELD_NAME",
		Operator:   "RULE_MATCH_OPERATOR_CONTAINS",
		Pattern:    "netflix",
	}}, 20)
	if err != nil {
		t.Fatalf("buildRuleQuery: %v", err)
	}
	if !strings.Contains(query, "LOWER(t.name) LIKE") || !strings.HasSuffix(query, "LIMIT 20") {
		t.Fatalf("unexpected query: %s", query)
	}
	if len(args) != 2 || args[0] != "u1" || args[1] != "netflix" {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestBuildRuleQueryNumericAndCategory(t *testing.T) {
	query, _, err := buildRuleQuery("u1", "AND", []RuleCondData{
		{MatchField: "RULE_MATCH_FIELD_AMOUNT", Operator: "RULE_MATCH_OPERATOR_GREATER_THAN", Pattern: "100"},
		{MatchField: "RULE_MATCH_FIELD_CATEGORY", Operator: "RULE_MATCH_OPERATOR_CONTAINS", Pattern: "food"},
	}, 5)
	if err != nil {
		t.Fatalf("buildRuleQuery: %v", err)
	}
	if !strings.Contains(query, "t.amount > ?") {
		t.Fatalf("expected amount clause, got: %s", query)
	}
	if !strings.Contains(query, "EXISTS (SELECT 1 FROM transaction_categories") || !strings.Contains(query, "LOWER(c.name) LIKE") {
		t.Fatalf("expected category EXISTS clause, got: %s", query)
	}
	if !strings.Contains(query, ") AND (") {
		t.Fatalf("expected AND join, got: %s", query)
	}
}

func TestBuildRuleQueryLikeEscapingAndRegex(t *testing.T) {
	_, args, err := buildRuleQuery("u1", "OR", []RuleCondData{{
		MatchField: "RULE_MATCH_FIELD_NAME",
		Operator:   "RULE_MATCH_OPERATOR_CONTAINS",
		Pattern:    "50%_off",
	}}, 20)
	if err != nil {
		t.Fatalf("buildRuleQuery: %v", err)
	}
	if args[1] != `50\%\_off` {
		t.Fatalf("expected escaped pattern, got %v", args[1])
	}

	query, _, err := buildRuleQuery("u1", "OR", []RuleCondData{{
		MatchField: "RULE_MATCH_FIELD_NAME",
		Operator:   "RULE_MATCH_OPERATOR_REGEX",
		Pattern:    "^net",
	}}, 20)
	if err != nil {
		t.Fatalf("buildRuleQuery: %v", err)
	}
	if !strings.Contains(query, "regexp(?, t.name)") {
		t.Fatalf("expected regexp clause, got: %s", query)
	}
}
