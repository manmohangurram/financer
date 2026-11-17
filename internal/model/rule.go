package model

import "encoding/json"

// jsonRaw is a json.RawMessage alias used for fields with flexible input shapes.
type jsonRaw = json.RawMessage

// RuleLogic is OR / AND.
type RuleLogic string

const (
	RuleLogicOr  RuleLogic = "OR"
	RuleLogicAnd RuleLogic = "AND"
)

// MatchField is what a condition matches against.
type MatchField string

const (
	MatchFieldName     MatchField = "NAME"
	MatchFieldAmount   MatchField = "AMOUNT"
	MatchFieldType     MatchField = "TYPE"
	MatchFieldCategory MatchField = "CATEGORY"
	MatchFieldAccount  MatchField = "ACCOUNT"
)

// MatchOperator is how a condition pattern is compared.
type MatchOperator string

const (
	MatchOperatorContains    MatchOperator = "CONTAINS"
	MatchOperatorStartsWith  MatchOperator = "STARTS_WITH"
	MatchOperatorEndsWith    MatchOperator = "ENDS_WITH"
	MatchOperatorEquals      MatchOperator = "EQUALS"
	MatchOperatorGreaterThan MatchOperator = "GREATER_THAN"
	MatchOperatorLessThan    MatchOperator = "LESS_THAN"
	MatchOperatorRegex       MatchOperator = "REGEX"
)

// ActionOp is how SET_NAME modifies the current name.
type ActionOp string

const (
	ActionOpRename    ActionOp = "RENAME"
	ActionOpAddPrefix ActionOp = "ADD_PREFIX"
	ActionOpAddSuffix ActionOp = "ADD_SUFFIX"
)

// ActionType is the kind of a stored rule action.
type ActionType string

const (
	ActionTypeSetName            ActionType = "SET_NAME"
	ActionTypeSetCategory        ActionType = "SET_CATEGORY"
	ActionTypeSetTransferAccount ActionType = "SET_TRANSFER_ACCOUNT"
)

// RuleCondition is one condition in the wire shape.
type RuleCondition struct {
	MatchField MatchField    `json:"matchField"`
	Operator   MatchOperator `json:"operator"`
	Pattern    string        `json:"pattern"`
}

// RuleAction is one output action in the wire shape.
type RuleAction struct {
	SetName              string    `json:"setName,omitempty"`
	SetNameOp            *ActionOp `json:"setNameOp,omitempty"`
	SetCategoryID        string    `json:"setCategoryId,omitempty"`
	SetTransferAccountID string    `json:"setTransferAccountId,omitempty"`
}

// RuleResponse is a rule as returned to the frontend.
type RuleResponse struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Priority   int64           `json:"priority"`
	Logic      RuleLogic       `json:"logic"`
	Conditions []RuleCondition `json:"conditions"`
	Actions    []RuleAction    `json:"actions"`
	CreatedAt  string          `json:"createdAt"`
}

// ReqRule is the create/update rule request.
type ReqRule struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Priority   int64           `json:"priority"`
	Logic      RuleLogic       `json:"logic"`
	Conditions []RuleCondition `json:"conditions"`
	Actions    []RuleAction    `json:"actions"`
}

// ReqPreview is the rules preview request.
type ReqPreview struct {
	Logic      RuleLogic       `json:"logic"`
	Conditions []RuleCondition `json:"conditions"`
	Limit      int64           `json:"limit"`
}

// WireRuleList is the GET /api/rules response.
type WireRuleList struct {
	Rules []*RuleResponse `json:"rules"`
}

// WirePreview is the POST /api/rules/preview response.
type WirePreview struct {
	Transactions []WireTxnPreview `json:"transactions"`
}

// WireRuleRun is the POST /api/rules/{id}/run response.
type WireRuleRun struct {
	Updated int64 `json:"updated"`
	Matched int64 `json:"matched"`
	Linked  int64 `json:"linked"`
	Created int64 `json:"created"`
}
