package repo

// RuleRow is a rule as stored by the repository layer (CreatedAt is the stored
// UTC form; services render it for the wire).
type RuleRow struct {
	ID         string
	Name       string
	Priority   int64
	Logic      string
	Conditions []RuleConditionRow
	Actions    []RuleActionRow
	CreatedAt  string
}

// RuleConditionRow mirrors model.RuleCondition without importing model.
type RuleConditionRow struct {
	MatchField string
	Operator   string
	Pattern    string
}

// RuleActionRow mirrors model.RuleAction without importing model.
type RuleActionRow struct {
	ActionType        string
	NameOp            string
	Value             string
	CategoryID        string
	TransferAccountID string
}

// OverlayRule is the minimal rule data needed to match and apply actions.
type OverlayRule struct {
	Logic      string
	Conditions []RuleConditionRow
	Actions    []RuleActionRow
}

// RuleRepo is the backend-agnostic rules repository.
type RuleCursor struct {
	Priority int64
	Name     string
	ID       string
}

type RuleRepo interface {
	Create(userID, name string, priority int64, logic string, conditions []RuleConditionRow, actions []RuleActionRow) (*RuleRow, error)
	Update(userID, id, name string, priority int64, logic string, conditions []RuleConditionRow, actions []RuleActionRow) (bool, error)
	Delete(userID, id string) (bool, error)
	GetByID(userID, id string) (*RuleRow, error)
	List(userID string) ([]*RuleRow, error)
	ListPage(userID string, limit int64, cursor *RuleCursor) ([]*RuleRow, string, error)
	ListForOverlay(userID string) ([]*OverlayRule, error)
}
