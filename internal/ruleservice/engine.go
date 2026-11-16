package ruleservice

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
)

// TransactionMatchData is everything a rule condition can match against.
type TransactionMatchData struct {
	Name       string
	Amount     float64
	Type       string
	AccountID  string
	Categories []string
}

// FormatAmount mirrors Go's strconv.FormatFloat(v, 'f', -1, 64).
func FormatAmount(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// MatchRuleData evaluates a rule's conditions against a transaction.
func MatchRuleData(txn *TransactionMatchData, logic string, conditions []repo.RuleConditionRow) bool {
	if len(conditions) == 0 {
		return false
	}
	isOr := logic != string(model.RuleLogicAnd)
	for _, cond := range conditions {
		matched := matchCondition(txn, cond)
		if isOr && matched {
			return true
		}
		if !isOr && !matched {
			return false
		}
	}
	return !isOr
}

func matchCondition(txn *TransactionMatchData, cond repo.RuleConditionRow) bool {
	for _, val := range fieldValues(txn, cond.MatchField) {
		if evaluate(val, cond.Operator, cond.Pattern) {
			return true
		}
	}
	return false
}

func fieldValues(txn *TransactionMatchData, field string) []string {
	switch field {
	case string(model.MatchFieldName):
		return []string{txn.Name}
	case string(model.MatchFieldAmount):
		return []string{FormatAmount(txn.Amount)}
	case string(model.MatchFieldType):
		return []string{txn.Type}
	case string(model.MatchFieldCategory):
		return txn.Categories
	case string(model.MatchFieldAccount):
		return []string{txn.AccountID}
	}
	return nil
}

func evaluate(value, op, pattern string) bool {
	switch op {
	case string(model.MatchOperatorContains):
		return strings.Contains(strings.ToLower(value), strings.ToLower(pattern))
	case string(model.MatchOperatorStartsWith):
		return strings.HasPrefix(strings.ToLower(value), strings.ToLower(pattern))
	case string(model.MatchOperatorEndsWith):
		return strings.HasSuffix(strings.ToLower(value), strings.ToLower(pattern))
	case string(model.MatchOperatorEquals):
		return strings.EqualFold(value, pattern)
	case string(model.MatchOperatorGreaterThan):
		return cmpFloat(value, pattern, func(a, b float64) bool { return a > b })
	case string(model.MatchOperatorLessThan):
		return cmpFloat(value, pattern, func(a, b float64) bool { return a < b })
	case string(model.MatchOperatorRegex):
		re, err := compileRuleRegex(pattern)
		return err == nil && re.MatchString(value)
	}
	return false
}

func cmpFloat(value, pattern string, cmp func(a, b float64) bool) bool {
	v, err1 := strconv.ParseFloat(value, 64)
	p, err2 := strconv.ParseFloat(pattern, 64)
	if err1 != nil || err2 != nil {
		return false
	}
	return cmp(v, p)
}

// compileRuleRegex compiles case-insensitively unless the pattern sets its own
// flags.
func compileRuleRegex(pattern string) (*regexp.Regexp, error) {
	pat := pattern
	if !strings.HasPrefix(pattern, "(?") {
		pat = "(?i)" + pattern
	}
	return regexp.Compile(pat)
}

// ApplyRuleActions applies a rule's output actions to a view in order.
func ApplyRuleActions(txn *repo.TransactionView, actions []repo.RuleActionRow) {
	for _, act := range actions {
		switch act.ActionType {
		case string(model.ActionTypeSetTransferAccount):
			continue
		case string(model.ActionTypeSetCategory):
			if act.CategoryID != "" {
				txn.CategoryIDs = []string{act.CategoryID}
			}
			continue
		default: // SET_NAME
			if act.Value == "" {
				continue
			}
			txn.Name = applyNameOp(txn.Name, act.Value, act.NameOp)
		}
	}
}

func applyNameOp(current, value, op string) string {
	switch op {
	case string(model.ActionOpAddPrefix):
		return value + current
	case string(model.ActionOpAddSuffix):
		return current + value
	default:
		return value
	}
}
