package services

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/mohan9182/financer/auth"
	"github.com/mohan9182/financer/api"
	"github.com/mohan9182/financer/repository"
)

type RuleService struct {
	ruleRepo *repository.RuleRepository
	catRepo   *repository.CategoryRepository
	txnRepo   *repository.TransactionRepository
}

func NewRuleService(ruleRepo *repository.RuleRepository, catRepo *repository.CategoryRepository, txnRepo *repository.TransactionRepository) *RuleService {
	return &RuleService{ruleRepo: ruleRepo, catRepo: catRepo, txnRepo: txnRepo}
}

func (s *RuleService) CreateRule(ctx context.Context, msg *api.CreateRuleRequest) (*api.RuleResponse, error) {
	if msg.Name == "" {
		return nil, BadRequest("name is required")
	}
	if len(msg.Conditions) == 0 {
		return nil, BadRequest("at least one condition is required")
	}

	conditions := repository.ConditionsToData(msg.Conditions)

	logic := repository.LogicString(msg.Logic)

	userID := ctx.Value(auth.UserIDKey).(string)
	rule, err := s.ruleRepo.Create(ctx, userID, msg.Name, msg.Priority, logic, conditions, msg.Actions)
	if err != nil {
		return nil, Conflict("%v", err)
	}

	return rule, nil
}

func (s *RuleService) UpdateRule(ctx context.Context, msg *api.UpdateRuleRequest) (*api.RuleResponse, error) {
	conditions := repository.ConditionsToData(msg.Conditions)

	logic := repository.LogicString(msg.Logic)

	err := s.ruleRepo.Update(ctx, msg.Id, msg.Name, msg.Priority, logic, conditions, msg.Actions)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, NotFound("rule %s not found", msg.Id)
		}
		return nil, ServerError("%v", err)
	}

	rule, err := s.ruleRepo.GetByID(ctx, msg.Id)
	if err != nil || rule == nil {
		return nil, ServerError("fetching updated rule")
	}

	return rule, nil
}

func (s *RuleService) DeleteRule(ctx context.Context, msg *api.DeleteRuleRequest) (*api.OperationResponse, error) {
	deleted, err := s.ruleRepo.Delete(ctx, msg.Id)
	if err != nil {
		return nil, ServerError("%v", err)
	}
	if !deleted {
		return nil, NotFound("rule %s not found", msg.Id)
	}

	return &api.OperationResponse{
		Success: true,
		Message: "rule deleted",
	}, nil
}

func (s *RuleService) ListRules(ctx context.Context, _ *api.ListRulesRequest) (*api.ListRulesResponse, error) {
	userID := ctx.Value(auth.UserIDKey).(string)
	rules, err := s.ruleRepo.List(ctx, userID)
	if err != nil {
		return nil, ServerError("%v", err)
	}

	return &api.ListRulesResponse{Rules: rules}, nil
}

// Overlay applies every matching rule's action to the transaction, in priority
// order (highest first). Later (lower-priority) rules can override earlier
// setters. Computed at read time only — nothing is persisted, so deleting a
// rule reverts transactions automatically.
func (s *RuleService) Overlay(ctx context.Context, txns []*api.TransactionResponse) error {
	if len(txns) == 0 {
		return nil
	}
	userID := ctx.Value(auth.UserIDKey).(string)

	rules, err := s.ruleRepo.ListForOverlay(ctx, userID)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		return nil
	}

	catNames, err := s.catNameByID(ctx, userID)
	if err != nil {
		return err
	}

	for _, txn := range txns {
		data := txnForMatchData{
			Name:       txn.Name,
			Amount:     float64(txn.Amount),
			Type:       int(txn.Type),
			AccountID:  txn.AccountId,
			Categories: categoryNames(catNames, txn.CategoryIds),
		}
		for _, rule := range rules {
			if !matchRuleData(data, rule.Logic, rule.Conditions) {
				continue
			}
			applyRuleActions(txn, rule.Actions)
		}
	}
	return nil
}

// compileRuleRegex compiles a rule regex case-insensitively, matching the
// case-insensitive behavior of the other string operators.
func compileRuleRegex(pattern string) (*regexp.Regexp, error) {
	pat := pattern
	if !strings.HasPrefix(pat, "(?") {
		pat = "(?i)" + pat
	}
	return regexp.Compile(pat)
}

// applyRuleActions applies a rule's output actions to a transaction in order.
// SET_TRANSFER_ACCOUNT is a write-time action with no view change, so it is skipped.
func applyRuleActions(txn *api.TransactionResponse, actions []*api.RuleAction) {
	for _, act := range actions {
		if act.SetTransferAccountId != "" {
			continue
		}
		if act.SetCategoryId != "" {
			txn.CategoryIds = []string{act.SetCategoryId}
			continue
		}
		if act.SetName == "" {
			continue
		}
		txn.Name = applyNameOp(txn.Name, act.SetName, act.SetNameOp)
	}
}

func (s *RuleService) Preview(ctx context.Context, msg *api.PreviewRuleRequest) ([]*api.TransactionResponse, error) {
	userID := ctx.Value(auth.UserIDKey).(string)

	conds := repository.ConditionsToData(msg.Conditions)
	for _, c := range conds {
		if c.Operator == "RULE_MATCH_OPERATOR_REGEX" {
			if _, err := compileRuleRegex(c.Pattern); err != nil {
				return nil, BadRequest("invalid regex in condition: %v", err)
			}
		}
	}

	limit := int32(msg.Limit)
	if limit <= 0 || limit > 20 {
		limit = 20
	}

	return s.txnRepo.SearchByRule(ctx, userID, repository.LogicString(msg.Logic), conds, limit)
}

func (s *RuleService) catNameByID(ctx context.Context, userID string) (map[string]string, error) {
	result, err := s.catRepo.List(ctx, userID, 0, "")
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(result.Categories))
	for _, c := range result.Categories {
		names[c.Id] = c.Name
	}
	return names, nil
}

func categoryNames(byID map[string]string, ids []string) []string {
	var names []string
	for _, id := range ids {
		if n, ok := byID[id]; ok {
			names = append(names, n)
		}
	}
	return names
}

// applyNameOp renders the rule's name output. RENAME is the default and
// replaces the name entirely; ADD_PREFIX / ADD_SUFFIX prepend/append.
func applyNameOp(current, value string, op api.RuleActionOp) string {
	switch op {
	case api.RuleActionOp_RULE_ACTION_OP_ADD_PREFIX:
		return value + current
	case api.RuleActionOp_RULE_ACTION_OP_ADD_SUFFIX:
		return current + value
	default:
		return value
	}
}

type txnForMatchData struct {
	Name       string
	Amount     float64
	Type       int
	AccountID  string
	Categories []string
}

func matchRuleData(txn txnForMatchData, logic string, conditions []repository.RuleCondData) bool {
	if len(conditions) == 0 {
		return false
	}

	isOr := strings.EqualFold(logic, "OR")

	for _, cond := range conditions {
		matched := matchConditionData(txn, cond)
		if isOr && matched {
			return true
		}
		if !isOr && !matched {
			return false
		}
	}

	return !isOr
}

func matchConditionData(txn txnForMatchData, cond repository.RuleCondData) bool {
	fieldValues := getFieldValuesData(txn, cond.MatchField)
	for _, val := range fieldValues {
		if evaluateConditionData(val, cond.Operator, cond.Pattern) {
			return true
		}
	}
	return false
}

func getFieldValuesData(txn txnForMatchData, field string) []string {
	switch field {
	case "RULE_MATCH_FIELD_NAME":
		return []string{txn.Name}
	case "RULE_MATCH_FIELD_AMOUNT":
		return []string{strconv.FormatFloat(txn.Amount, 'f', -1, 64)}
	case "RULE_MATCH_FIELD_TYPE":
		return []string{strconv.Itoa(txn.Type)}
	case "RULE_MATCH_FIELD_CATEGORY":
		if len(txn.Categories) == 0 {
			return []string{}
		}
		return txn.Categories
	case "RULE_MATCH_FIELD_ACCOUNT":
		return []string{txn.AccountID}
	default:
		return nil
	}
}

func evaluateConditionData(value, operator, pattern string) bool {
	switch operator {
	case "RULE_MATCH_OPERATOR_CONTAINS":
		return strings.Contains(strings.ToLower(value), strings.ToLower(pattern))
	case "RULE_MATCH_OPERATOR_STARTS_WITH":
		return strings.HasPrefix(strings.ToLower(value), strings.ToLower(pattern))
	case "RULE_MATCH_OPERATOR_ENDS_WITH":
		return strings.HasSuffix(strings.ToLower(value), strings.ToLower(pattern))
	case "RULE_MATCH_OPERATOR_EQUALS":
		return strings.EqualFold(value, pattern)
	case "RULE_MATCH_OPERATOR_GREATER_THAN":
		val, err1 := strconv.ParseFloat(value, 64)
		pat, err2 := strconv.ParseFloat(pattern, 64)
		if err1 != nil || err2 != nil {
			return false
		}
		return val > pat
	case "RULE_MATCH_OPERATOR_LESS_THAN":
		val, err1 := strconv.ParseFloat(value, 64)
		pat, err2 := strconv.ParseFloat(pattern, 64)
		if err1 != nil || err2 != nil {
			return false
		}
		return val < pat
	case "RULE_MATCH_OPERATOR_REGEX":
		re, err := compileRuleRegex(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(value)
	default:
		return false
	}
}
