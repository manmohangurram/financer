package sqlite

import (
	"database/sql"

	"github.com/google/uuid"

	"github.com/mohan9182/financer/internal/repo"
	tim "github.com/mohan9182/financer/internal/time"
)

// RuleRepo implements repo.RuleRepo over SQLite.
type RuleRepo struct {
	db *sql.DB
}

func NewRuleRepo(db *sql.DB) *RuleRepo {
	return &RuleRepo{db: db}
}

func (r *RuleRepo) Create(userID, name string, priority int64, logic string, conditions []repo.RuleConditionRow, actions []repo.RuleActionRow) (*repo.RuleRow, error) {
	id := uuid.New().String()
	now := tim.NowGoTS()
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		"INSERT INTO rules (id, name, priority, logic, created_at, user_id) VALUES (?, ?, ?, ?, ?, ?)",
		id, name, priority, logic, now, userID,
	); err != nil {
		return nil, err
	}
	if err := insertConditions(tx, id, conditions); err != nil {
		return nil, err
	}
	if err := insertActions(tx, id, actions); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &repo.RuleRow{
		ID: id, Name: name, Priority: priority, Logic: logic,
		Conditions: conditions, Actions: actions, CreatedAt: now,
	}, nil
}

func (r *RuleRepo) Update(userID, id, name string, priority int64, logic string, conditions []repo.RuleConditionRow, actions []repo.RuleActionRow) (bool, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(
		"UPDATE rules SET name = COALESCE(NULLIF(?, ''), name), priority = ?, logic = ? WHERE id = ? AND user_id = ?",
		name, priority, logic, id, userID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// Either not found or unchanged; confirm existence.
		var exists bool
		if err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM rules WHERE id = ? AND user_id = ?)", id, userID).Scan(&exists); err != nil {
			return false, err
		}
		if !exists {
			return false, nil
		}
	}
	if _, err := tx.Exec("DELETE FROM rule_conditions WHERE rule_id = ?", id); err != nil {
		return false, err
	}
	if _, err := tx.Exec("DELETE FROM rule_actions WHERE rule_id = ?", id); err != nil {
		return false, err
	}
	if err := insertConditions(tx, id, conditions); err != nil {
		return false, err
	}
	if err := insertActions(tx, id, actions); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (r *RuleRepo) Delete(userID, id string) (bool, error) {
	res, err := r.db.Exec("DELETE FROM rules WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *RuleRepo) GetByID(userID, id string) (*repo.RuleRow, error) {
	var row repo.RuleRow
	err := r.db.QueryRow(
		"SELECT id, name, priority, logic, created_at FROM rules WHERE id = ? AND user_id = ?", id, userID,
	).Scan(&row.ID, &row.Name, &row.Priority, &row.Logic, &row.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.Conditions, err = r.conditionsForRule(id); err != nil {
		return nil, err
	}
	if row.Actions, err = r.actionsForRule(id); err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *RuleRepo) List(userID string) ([]*repo.RuleRow, error) {
	rows, err := r.db.Query(
		"SELECT id, name, priority, logic, created_at FROM rules WHERE user_id = ? ORDER BY priority DESC, name ASC", userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []*repo.RuleRow
	for rows.Next() {
		var row repo.RuleRow
		if err := rows.Scan(&row.ID, &row.Name, &row.Priority, &row.Logic, &row.CreatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, &row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, row := range rules {
		if row.Conditions, err = r.conditionsForRule(row.ID); err != nil {
			return nil, err
		}
		if row.Actions, err = r.actionsForRule(row.ID); err != nil {
			return nil, err
		}
	}
	return rules, nil
}

func (r *RuleRepo) ListForOverlay(userID string) ([]*repo.OverlayRule, error) {
	rules, err := r.List(userID)
	if err != nil {
		return nil, err
	}
	out := make([]*repo.OverlayRule, 0, len(rules))
	for _, rule := range rules {
		out = append(out, &repo.OverlayRule{
			Logic:      rule.Logic,
			Conditions: rule.Conditions,
			Actions:    rule.Actions,
		})
	}
	return out, nil
}

func (r *RuleRepo) conditionsForRule(ruleID string) ([]repo.RuleConditionRow, error) {
	rows, err := r.db.Query(
		"SELECT match_field, operator, pattern FROM rule_conditions WHERE rule_id = ?", ruleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repo.RuleConditionRow
	for rows.Next() {
		var c repo.RuleConditionRow
		if err := rows.Scan(&c.MatchField, &c.Operator, &c.Pattern); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *RuleRepo) actionsForRule(ruleID string) ([]repo.RuleActionRow, error) {
	rows, err := r.db.Query(
		"SELECT action_type, name_op, value, category_id, transfer_account_id FROM rule_actions WHERE rule_id = ? ORDER BY rowid", ruleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repo.RuleActionRow
	for rows.Next() {
		var a repo.RuleActionRow
		if err := rows.Scan(&a.ActionType, &a.NameOp, &a.Value, &a.CategoryID, &a.TransferAccountID); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func insertConditions(tx *sql.Tx, ruleID string, conditions []repo.RuleConditionRow) error {
	for _, cond := range conditions {
		if _, err := tx.Exec(
			"INSERT INTO rule_conditions (id, rule_id, match_field, operator, pattern) VALUES (?, ?, ?, ?, ?)",
			uuid.New().String(), ruleID, cond.MatchField, cond.Operator, cond.Pattern,
		); err != nil {
			return err
		}
	}
	return nil
}

func insertActions(tx *sql.Tx, ruleID string, actions []repo.RuleActionRow) error {
	for _, act := range actions {
		if _, err := tx.Exec(
			"INSERT INTO rule_actions (id, rule_id, action_type, name_op, value, category_id, transfer_account_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
			uuid.New().String(), ruleID, act.ActionType, act.NameOp, act.Value, act.CategoryID, act.TransferAccountID,
		); err != nil {
			return err
		}
	}
	return nil
}
