// Package ruleservice implements rule CRUD, the rule matching engine, the
// write-time overlay that snapshots a resolved clean_name/category, preview,
// and the "run now" backfill.
package ruleservice

import (
	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
	tim "github.com/mohan9182/financer/internal/time"
)

type Service struct {
	rules      repo.RuleRepo
	categories repo.CategoryRepo
	txns       repo.TransactionRepo
}

func New(rules repo.RuleRepo, categories repo.CategoryRepo, txns repo.TransactionRepo) *Service {
	return &Service{rules: rules, categories: categories, txns: txns}
}

// Create validates and stores a rule.
func (s *Service) Create(userID, name string, priority int64, logic string, conditions []model.RuleCondition, actions []model.RuleAction) (*model.RuleResponse, error) {
	if name == "" {
		return nil, errors.BadRequest("name is required")
	}
	if len(conditions) == 0 {
		return nil, errors.BadRequest("at least one condition is required")
	}
	row, err := s.rules.Create(userID, name, priority, logic,
		toConditionRows(conditions), toActionRows(actions))
	if err != nil {
		return nil, err
	}
	return toRuleResponse(row), nil
}

// Update replaces a rule's conditions and actions.
func (s *Service) Update(userID, id, name string, priority int64, logic string, conditions []model.RuleCondition, actions []model.RuleAction) (*model.RuleResponse, error) {
	updated, err := s.rules.Update(userID, id, name, priority, logic,
		toConditionRows(conditions), toActionRows(actions))
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, errors.NotFound("rule " + id + " not found")
	}
	row, err := s.rules.GetByID(userID, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errors.Internal("fetching updated rule")
	}
	return toRuleResponse(row), nil
}

func (s *Service) Delete(userID, id string) error {
	ok, err := s.rules.Delete(userID, id)
	if err != nil {
		return err
	}
	if !ok {
		return errors.NotFound("rule " + id + " not found")
	}
	return nil
}

func (s *Service) List(userID string) ([]*model.RuleResponse, error) {
	rows, err := s.rules.List(userID)
	if err != nil {
		return nil, err
	}
	out := make([]*model.RuleResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRuleResponse(row))
	}
	return out, nil
}

// Overlay applies every matching rule's actions to each view, in priority
// order; later rules override earlier setters. Nothing is persisted.
func (s *Service) Overlay(userID string, views []repo.TransactionView) error {
	if len(views) == 0 {
		return nil
	}
	rules, err := s.rules.ListForOverlay(userID)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		return nil
	}
	catNames, err := s.categoryNameByID(userID)
	if err != nil {
		return err
	}
	for i := range views {
		data := txnMatchData(&views[i], catNames)
		for _, rule := range rules {
			if !MatchRuleData(data, rule.Logic, rule.Conditions) {
				continue
			}
			ApplyRuleActions(&views[i], rule.Actions)
		}
	}
	return nil
}

// Preview matches conditions against existing transactions.
func (s *Service) Preview(userID string, logic string, conditions []model.RuleCondition, limit int64) ([]*repo.TransactionView, error) {
	conds := toConditionRows(conditions)
	for _, c := range conds {
		if c.Operator == string(model.MatchOperatorRegex) {
			if _, err := compileRuleRegex(c.Pattern); err != nil {
				return nil, errors.BadRequest("invalid regex in condition: " + err.Error())
			}
		}
	}
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	rows, err := s.txns.List(userID, &repo.TransactionListFilter{})
	if err != nil {
		return nil, err
	}
	catNames, err := s.categoryNameByID(userID)
	if err != nil {
		return nil, err
	}
	var out []*repo.TransactionView
	for _, row := range rows.Rows {
		view := rowToView(row)
		data := txnMatchData(view, catNames)
		if MatchRuleData(data, logic, conds) {
			out = append(out, view)
			if int64(len(out)) >= limit {
				break
			}
		}
	}
	return out, nil
}

// ApplyToExisting applies every rule to existing rows, persisting the resolved
// name and categories.
func (s *Service) ApplyToExisting(userID string) (int64, error) {
	rules, err := s.rules.ListForOverlay(userID)
	if err != nil {
		return 0, err
	}
	return s.applyRules(userID, rules)
}

// Run applies one rule to existing rows (the "run now" action).
func (s *Service) Run(userID, ruleID string) (int64, error) {
	rule, err := s.rules.GetByID(userID, ruleID)
	if err != nil {
		return 0, err
	}
	if rule == nil {
		return 0, errors.NotFound("rule " + ruleID + " not found")
	}
	overlay := &repo.OverlayRule{
		Logic:      rule.Logic,
		Conditions: rule.Conditions,
		Actions:    rule.Actions,
	}
	return s.applyRules(userID, []*repo.OverlayRule{overlay})
}

func (s *Service) applyRules(userID string, rules []*repo.OverlayRule) (int64, error) {
	if len(rules) == 0 {
		return 0, nil
	}
	catNames, err := s.categoryNameByID(userID)
	if err != nil {
		return 0, err
	}
	rows, err := s.txns.List(userID, &repo.TransactionListFilter{})
	if err != nil {
		return 0, err
	}
	var changed int64
	for _, row := range rows.Rows {
		view := rowToView(row)
		data := txnMatchData(view, catNames)
		for _, rule := range rules {
			if MatchRuleData(data, rule.Logic, rule.Conditions) {
				ApplyRuleActions(view, rule.Actions)
			}
		}
		var clean *string
		if view.Name != row.Txn.Name {
			name := view.Name
			clean = &name
		}
		if clean != nil || !equalStrings(view.CategoryIDs, row.CategoryIDs) {
			if err := s.txns.ApplyRuleResult(userID, row.Txn.ID, clean, view.CategoryIDs); err != nil {
				return changed, err
			}
			changed++
		}
	}
	return changed, nil
}

func (s *Service) categoryNameByID(userID string) (map[string]string, error) {
	res, err := s.categories.List(userID, 0, "")
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(res.Categories))
	for _, c := range res.Categories {
		out[c.ID] = c.Name
	}
	return out, nil
}

func txnMatchData(txn *repo.TransactionView, catNames map[string]string) *TransactionMatchData {
	categories := make([]string, 0, len(txn.CategoryIDs))
	for _, id := range txn.CategoryIDs {
		if name, ok := catNames[id]; ok {
			categories = append(categories, name)
		}
	}
	return &TransactionMatchData{
		Name:       txn.Name,
		Amount:     txn.Amount,
		Type:       txn.Type,
		AccountID:  txn.AccountID,
		Categories: categories,
	}
}

func rowToView(row repo.ListRow) *repo.TransactionView {
	return &repo.TransactionView{
		Name:        row.Txn.Name,
		Amount:      row.Txn.Amount,
		Type:        row.Txn.Type,
		AccountID:   row.Txn.AccountID,
		CategoryIDs: row.CategoryIDs,
		OccurredAt:  row.Txn.OccurredAt,
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func toConditionRows(conditions []model.RuleCondition) []repo.RuleConditionRow {
	out := make([]repo.RuleConditionRow, 0, len(conditions))
	for _, c := range conditions {
		out = append(out, repo.RuleConditionRow{
			MatchField: string(c.MatchField),
			Operator:   string(c.Operator),
			Pattern:    c.Pattern,
		})
	}
	return out
}

func toActionRows(actions []model.RuleAction) []repo.RuleActionRow {
	out := make([]repo.RuleActionRow, 0, len(actions))
	for _, a := range actions {
		switch {
		case a.SetTransferAccountID != "":
			out = append(out, repo.RuleActionRow{
				ActionType:        string(model.ActionTypeSetTransferAccount),
				TransferAccountID: a.SetTransferAccountID,
			})
		case a.SetCategoryID != "":
			out = append(out, repo.RuleActionRow{
				ActionType: string(model.ActionTypeSetCategory),
				CategoryID: a.SetCategoryID,
			})
		default:
			op := string(model.ActionOpRename)
			if a.SetNameOp != nil {
				op = string(*a.SetNameOp)
			}
			out = append(out, repo.RuleActionRow{
				ActionType: string(model.ActionTypeSetName),
				NameOp:     op,
				Value:      a.SetName,
			})
		}
	}
	return out
}

func toRuleResponse(row *repo.RuleRow) *model.RuleResponse {
	conditions := make([]model.RuleCondition, 0, len(row.Conditions))
	for _, c := range row.Conditions {
		conditions = append(conditions, model.RuleCondition{
			MatchField: model.MatchField(c.MatchField),
			Operator:   model.MatchOperator(c.Operator),
			Pattern:    c.Pattern,
		})
	}
	actions := make([]model.RuleAction, 0, len(row.Actions))
	for _, a := range row.Actions {
		switch a.ActionType {
		case string(model.ActionTypeSetTransferAccount):
			actions = append(actions, model.RuleAction{SetTransferAccountID: a.TransferAccountID})
		case string(model.ActionTypeSetCategory):
			actions = append(actions, model.RuleAction{SetCategoryID: a.CategoryID})
		default:
			var op *model.ActionOp
			if a.NameOp != "" {
				o := model.ActionOp(a.NameOp)
				op = &o
			}
			actions = append(actions, model.RuleAction{SetName: a.Value, SetNameOp: op})
		}
	}
	return &model.RuleResponse{
		ID:         row.ID,
		Name:       row.Name,
		Priority:   row.Priority,
		Logic:      model.RuleLogic(row.Logic),
		Conditions: conditions,
		Actions:    actions,
		CreatedAt:  tim.TSRFC3339(row.CreatedAt),
	}
}
