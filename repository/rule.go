package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mohan9182/financer/api"
	"github.com/google/uuid"
)

type RuleRepository struct {
	*BaseRepository
}

func NewRuleRepository(base *BaseRepository) *RuleRepository {
	return &RuleRepository{BaseRepository: base}
}

type RuleRecord struct {
	ID        string
	Name      string
	Priority  int
	Logic     string
	CreatedAt time.Time
}

type RuleCondData struct {
	MatchField string
	Operator   string
	Pattern    string
}

func scanRule(row scannable) (*api.RuleResponse, error) {
	var rule api.RuleResponse
	var createdAt time.Time
	var logicStr string
	err := row.Scan(&rule.Id, &rule.Name, &rule.Priority, &logicStr, &createdAt)
	if err != nil {
		return nil, err
	}
	rule.Logic = parseLogicEnum(logicStr)
	rule.CreatedAt = createdAt
	return &rule, nil
}

func (r *RuleRepository) Create(ctx context.Context, userID, name string, priority int32, logic string, conditions []RuleCondData, actions []*api.RuleAction) (*api.RuleResponse, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	err := r.ExecInTx(ctx, func(tx *Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO rules (id, name, priority, logic, created_at, user_id) VALUES (?, ?, ?, ?, ?, ?)`,
			id, name, priority, logic, now, userID,
		); err != nil {
			return fmt.Errorf("inserting rule: %w", err)
		}

		for _, cond := range conditions {
			condID := uuid.New().String()
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO rule_conditions (id, rule_id, match_field, operator, pattern) VALUES (?, ?, ?, ?, ?)`,
				condID, id, cond.MatchField, cond.Operator, cond.Pattern,
			); err != nil {
				return fmt.Errorf("inserting rule condition: %w", err)
			}
		}

		if err := r.insertActions(ctx, tx, id, actions); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &api.RuleResponse{
		Id:         id,
		Name:       name,
		Priority:   priority,
		Logic:      parseLogicEnum(logic),
		Conditions: buildConditionProtos(conditions),
		Actions:    actions,
		CreatedAt:  now,
	}, nil
}

func (r *RuleRepository) insertActions(ctx context.Context, tx *Tx, id string, actions []*api.RuleAction) error {
	for _, act := range actions {
		actionID := uuid.New().String()
		actionType := "SET_NAME"
		var nameOp, value, catID, transferAccountID string
		if act.SetTransferAccountId != "" {
			actionType = "SET_TRANSFER_ACCOUNT"
			transferAccountID = act.SetTransferAccountId
		} else if act.SetCategoryId != "" {
			actionType = "SET_CATEGORY"
			catID = act.SetCategoryId
		} else {
			nameOp = act.SetNameOp.String()
			value = act.SetName
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO rule_actions (id, rule_id, action_type, name_op, value, category_id, transfer_account_id) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			actionID, id, actionType, nameOp, value, catID, transferAccountID,
		); err != nil {
			return fmt.Errorf("inserting rule action: %w", err)
		}
	}
	return nil
}

func (r *RuleRepository) GetByID(ctx context.Context, id string) (*api.RuleResponse, error) {
	rule, err := QueryOne(ctx, r.readDB,
		`SELECT id, name, priority, logic, created_at FROM rules WHERE id = ?`,
		scanRule, id,
	)
	if err != nil {
		return nil, fmt.Errorf("querying rule: %w", err)
	}
	if rule == nil {
		return nil, nil
	}

	rule.Conditions, _ = r.GetConditions(ctx, id)
	rule.Actions, _ = r.GetActions(ctx, id)
	return rule, nil
}

func (r *RuleRepository) Update(ctx context.Context, id, name string, priority int32, logic string, conditions []RuleCondData, actions []*api.RuleAction) error {
	return r.ExecInTx(ctx, func(tx *Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE rules SET name = COALESCE(NULLIF(?, ''), name), priority = ?, logic = ? WHERE id = ?`,
			name, priority, logic, id,
		)
		if err != nil {
			return fmt.Errorf("updating rule: %w", err)
		}

		rows, _ := result.RowsAffected()
		if rows == 0 {
			return sql.ErrNoRows
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM rule_conditions WHERE rule_id = ?`, id); err != nil {
			return fmt.Errorf("deleting old conditions: %w", err)
		}

		for _, cond := range conditions {
			condID := uuid.New().String()
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO rule_conditions (id, rule_id, match_field, operator, pattern) VALUES (?, ?, ?, ?, ?)`,
				condID, id, cond.MatchField, cond.Operator, cond.Pattern,
			); err != nil {
				return fmt.Errorf("inserting condition: %w", err)
			}
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM rule_actions WHERE rule_id = ?`, id); err != nil {
			return fmt.Errorf("deleting old actions: %w", err)
		}

		return r.insertActions(ctx, tx, id, actions)
	})
}

func (r *RuleRepository) Delete(ctx context.Context, id string) (bool, error) {
	result, err := r.ExecContext(ctx, `DELETE FROM rules WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("deleting rule: %w", err)
	}

	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

func (r *RuleRepository) List(ctx context.Context, userID string) ([]*api.RuleResponse, error) {
	results, err := QueryAll(ctx, r.readDB,
		`SELECT id, name, priority, logic, created_at FROM rules WHERE user_id = ? ORDER BY priority DESC, name ASC`,
		scanRule, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing rules: %w", err)
	}

	ids := make([]string, len(results))
	for i, rule := range results {
		ids[i] = rule.Id
	}
	conds, _ := r.conditionsByRule(ctx, ids)
	acts, _ := r.actionsByRule(ctx, ids)
	for _, rule := range results {
		rule.Conditions = conds[rule.Id]
		rule.Actions = acts[rule.Id]
	}
	return results, nil
}

func inPlaceholders(n int) string {
	ps := make([]string, n)
	for i := range ps {
		ps[i] = "?"
	}
	return strings.Join(ps, ",")
}

func (r *RuleRepository) conditionsByRule(ctx context.Context, ids []string) (map[string][]*api.RuleCondition, error) {
	out := make(map[string][]*api.RuleCondition)
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.readDB.QueryContext(ctx,
		`SELECT rule_id, match_field, operator, pattern FROM rule_conditions WHERE rule_id IN (`+inPlaceholders(len(ids))+`)`, toAny(ids)...)
	if err != nil {
		return nil, fmt.Errorf("querying conditions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var rid, mf, op, pattern string
		if err := rows.Scan(&rid, &mf, &op, &pattern); err != nil {
			return nil, fmt.Errorf("scanning condition: %w", err)
		}
		out[rid] = append(out[rid], &api.RuleCondition{
			MatchField: parseMatchFieldEnum(mf),
			Operator:   parseMatchOperatorEnum(op),
			Pattern:    pattern,
		})
	}
	return out, rows.Err()
}

func (r *RuleRepository) actionsByRule(ctx context.Context, ids []string) (map[string][]*api.RuleAction, error) {
	out := make(map[string][]*api.RuleAction)
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.readDB.QueryContext(ctx,
		`SELECT rule_id, action_type, name_op, value, category_id, transfer_account_id FROM rule_actions
		 WHERE rule_id IN (`+inPlaceholders(len(ids))+`) ORDER BY rule_id, rowid`, toAny(ids)...)
	if err != nil {
		return nil, fmt.Errorf("querying actions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var rid, actionType, nameOp, value, catID, transferAccountID string
		if err := rows.Scan(&rid, &actionType, &nameOp, &value, &catID, &transferAccountID); err != nil {
			return nil, fmt.Errorf("scanning action: %w", err)
		}
		a := &api.RuleAction{}
		if actionType == "SET_TRANSFER_ACCOUNT" {
			a.SetTransferAccountId = transferAccountID
		} else if actionType == "SET_CATEGORY" {
			a.SetCategoryId = catID
		} else {
			a.SetName = value
			a.SetNameOp = parseRuleActionOp(nameOp)
		}
		out[rid] = append(out[rid], a)
	}
	return out, rows.Err()
}

func toAny(ids []string) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

func (r *RuleRepository) GetConditions(ctx context.Context, ruleID string) ([]*api.RuleCondition, error) {
	rows, err := r.readDB.QueryContext(ctx,
		`SELECT match_field, operator, pattern FROM rule_conditions WHERE rule_id = ?`, ruleID)
	if err != nil {
		return nil, fmt.Errorf("querying conditions: %w", err)
	}
	defer rows.Close()

	var conditions []*api.RuleCondition
	for rows.Next() {
		var c api.RuleCondition
		var matchField, operator string
		if err := rows.Scan(&matchField, &operator, &c.Pattern); err != nil {
			return nil, fmt.Errorf("scanning condition: %w", err)
		}
		c.MatchField = parseMatchFieldEnum(matchField)
		c.Operator = parseMatchOperatorEnum(operator)
		conditions = append(conditions, &c)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return conditions, nil
}

func (r *RuleRepository) GetActions(ctx context.Context, ruleID string) ([]*api.RuleAction, error) {
	rows, err := r.readDB.QueryContext(ctx,
		`SELECT action_type, name_op, value, category_id, transfer_account_id FROM rule_actions WHERE rule_id = ? ORDER BY rowid`, ruleID)
	if err != nil {
		return nil, fmt.Errorf("querying actions: %w", err)
	}
	defer rows.Close()
	var actions []*api.RuleAction
	for rows.Next() {
		var actionType, nameOp, value, catID, transferAccountID string
		if err := rows.Scan(&actionType, &nameOp, &value, &catID, &transferAccountID); err != nil {
			return nil, fmt.Errorf("scanning action: %w", err)
		}
		a := &api.RuleAction{}
		if actionType == "SET_TRANSFER_ACCOUNT" {
			a.SetTransferAccountId = transferAccountID
		} else if actionType == "SET_CATEGORY" {
			a.SetCategoryId = catID
		} else {
			a.SetName = value
			a.SetNameOp = parseRuleActionOp(nameOp)
		}
		actions = append(actions, a)
	}
	return actions, rows.Err()
}

type OverlayRule struct {
	Logic      string
	Actions    []*api.RuleAction
	Conditions []RuleCondData
}

// ListForOverlay loads a user's rules ordered by priority (highest first) with
// their conditions and actions, for read-time compound mapping.
func (r *RuleRepository) ListForOverlay(ctx context.Context, userID string) ([]OverlayRule, error) {
	records, err := r.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	// List orders by priority DESC already.
	rules := make([]OverlayRule, 0, len(records))
	for _, rec := range records {
		rules = append(rules, OverlayRule{
			Logic:      LogicString(rec.Logic),
			Actions:    rec.Actions,
			Conditions: ConditionsToData(rec.Conditions),
		})
	}
	return rules, nil
}

func ConditionsToData(conds []*api.RuleCondition) []RuleCondData {
	out := make([]RuleCondData, 0, len(conds))
	for _, c := range conds {
		out = append(out, RuleCondData{
			MatchField: c.MatchField.String(),
			Operator:   c.Operator.String(),
			Pattern:    c.Pattern,
		})
	}
	return out
}

func parseRuleActionOp(s string) api.RuleActionOp {
	switch s {
	case "RULE_ACTION_OP_RENAME":
		return api.RuleActionOp_RULE_ACTION_OP_RENAME
	case "RULE_ACTION_OP_ADD_PREFIX":
		return api.RuleActionOp_RULE_ACTION_OP_ADD_PREFIX
	case "RULE_ACTION_OP_ADD_SUFFIX":
		return api.RuleActionOp_RULE_ACTION_OP_ADD_SUFFIX
	default:
		return api.RuleActionOp_RULE_ACTION_OP_UNSPECIFIED
	}
}

func parseLogicEnum(s string) api.RuleLogic {
	switch s {
	case "AND":
		return api.RuleLogic_RULE_LOGIC_AND
	default:
		return api.RuleLogic_RULE_LOGIC_OR
	}
}

func parseMatchFieldEnum(s string) api.RuleMatchField {
	switch s {
	case "RULE_MATCH_FIELD_NAME":
		return api.RuleMatchField_RULE_MATCH_FIELD_NAME
	case "RULE_MATCH_FIELD_AMOUNT":
		return api.RuleMatchField_RULE_MATCH_FIELD_AMOUNT
	case "RULE_MATCH_FIELD_TYPE":
		return api.RuleMatchField_RULE_MATCH_FIELD_TYPE
	case "RULE_MATCH_FIELD_CATEGORY":
		return api.RuleMatchField_RULE_MATCH_FIELD_CATEGORY
	case "RULE_MATCH_FIELD_ACCOUNT":
		return api.RuleMatchField_RULE_MATCH_FIELD_ACCOUNT
	default:
		return api.RuleMatchField_RULE_MATCH_FIELD_UNSPECIFIED
	}
}

func parseMatchOperatorEnum(s string) api.RuleMatchOperator {
	switch s {
	case "RULE_MATCH_OPERATOR_CONTAINS":
		return api.RuleMatchOperator_RULE_MATCH_OPERATOR_CONTAINS
	case "RULE_MATCH_OPERATOR_STARTS_WITH":
		return api.RuleMatchOperator_RULE_MATCH_OPERATOR_STARTS_WITH
	case "RULE_MATCH_OPERATOR_ENDS_WITH":
		return api.RuleMatchOperator_RULE_MATCH_OPERATOR_ENDS_WITH
	case "RULE_MATCH_OPERATOR_EQUALS":
		return api.RuleMatchOperator_RULE_MATCH_OPERATOR_EQUALS
	case "RULE_MATCH_OPERATOR_GREATER_THAN":
		return api.RuleMatchOperator_RULE_MATCH_OPERATOR_GREATER_THAN
	case "RULE_MATCH_OPERATOR_LESS_THAN":
		return api.RuleMatchOperator_RULE_MATCH_OPERATOR_LESS_THAN
	case "RULE_MATCH_OPERATOR_REGEX":
		return api.RuleMatchOperator_RULE_MATCH_OPERATOR_REGEX
	default:
		return api.RuleMatchOperator_RULE_MATCH_OPERATOR_UNSPECIFIED
	}
}

func buildConditionProtos(conditions []RuleCondData) []*api.RuleCondition {
	var result []*api.RuleCondition
	for _, c := range conditions {
		result = append(result, &api.RuleCondition{
			MatchField: parseMatchFieldEnum(c.MatchField),
			Operator:   parseMatchOperatorEnum(c.Operator),
			Pattern:    c.Pattern,
		})
	}
	return result
}

func LogicString(logic api.RuleLogic) string {
	if logic == api.RuleLogic_RULE_LOGIC_AND {
		return "AND"
	}
	return "OR"
}
