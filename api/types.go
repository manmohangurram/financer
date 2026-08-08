// Package api holds the API request/response types. Hand-written,
// no protobuf. Field shapes mirror what the frontend's fetch clients send/receive.
package api

import "time"

type SignupRequest struct {
	Email    string
	Password string
	Name     string
}

type LoginRequest struct {
	Email    string
	Password string
}

type RefreshTokenRequest struct {
	RefreshToken string
}

type AuthResponse struct {
	AccessToken  string
	RefreshToken string
	UserId       string
	Email        string
	Name         string
}

// --- user settings ---

type ProfileResponse struct {
	UserId    string
	Name      string
	Email     string
	AvatarUrl string
}

type UpdateProfileRequest struct {
	Name      string
	Email     string
	AvatarUrl string
}

type ChangePasswordRequest struct {
	CurrentPassword string
	NewPassword     string
}

type AccountType int32

const (
	AccountType_ACCOUNT_TYPE_UNSPECIFIED AccountType = 0
	AccountType_ACCOUNT_TYPE_CHECKING    AccountType = 1
	AccountType_ACCOUNT_TYPE_SAVINGS     AccountType = 2
	AccountType_ACCOUNT_TYPE_CREDIT_CARD AccountType = 3
	AccountType_ACCOUNT_TYPE_LOAN        AccountType = 4
)

var AccountType_value = map[string]int32{
	"ACCOUNT_TYPE_UNSPECIFIED": 0,
	"ACCOUNT_TYPE_CHECKING":    1,
	"ACCOUNT_TYPE_SAVINGS":     2,
	"ACCOUNT_TYPE_CREDIT_CARD": 3,
	"ACCOUNT_TYPE_LOAN":        4,
}

func (x AccountType) String() string {
	switch x {
	case AccountType_ACCOUNT_TYPE_CHECKING:
		return "ACCOUNT_TYPE_CHECKING"
	case AccountType_ACCOUNT_TYPE_SAVINGS:
		return "ACCOUNT_TYPE_SAVINGS"
	case AccountType_ACCOUNT_TYPE_CREDIT_CARD:
		return "ACCOUNT_TYPE_CREDIT_CARD"
	case AccountType_ACCOUNT_TYPE_LOAN:
		return "ACCOUNT_TYPE_LOAN"
	default:
		return "ACCOUNT_TYPE_UNSPECIFIED"
	}
}

type CreateAccountRequest struct {
	BankName        string
	AccountNickname string
	AccountType     AccountType
}

type UpdateAccountRequest struct {
	Id              string
	BankName        string
	AccountNickname string
	AccountType     AccountType
}

type DeleteAccountRequest struct {
	Id string
}

type ListAccountsRequest struct{}

type AccountResponse struct {
	Id              string
	BankName        string
	AccountNickname string
	Balance         float32
	AccountType     AccountType
	CreatedAt       time.Time
}

type ListAccountsResponse struct {
	Accounts []*AccountResponse
}

// --- categories ---

type ListCategoriesRequest struct {
	PageSize  int32
	PageToken string
}

type CreateCategoriesRequest struct {
	Categories []*CreateCategoryRequest
}

type CreateCategoryRequest struct {
	Name string
}

type UpdateCategoriesRequest struct {
	Categories []*UpdateCategoryRequest
}

type UpdateCategoryRequest struct {
	Id   string
	Name string
}

type DeleteCategoriesRequest struct {
	Ids []string
}

type CategoryResponse struct {
	Id        string
	Name      string
	CreatedAt time.Time
}

type ListCategoriesResponse struct {
	Categories    []*CategoryResponse
	NextPageToken string
}

type OperationResponse struct {
	Success bool
	Message string
}

type TransactionType int32

const (
	TransactionType_DEBIT  TransactionType = 0
	TransactionType_CREDIT TransactionType = 1
)

var TransactionType_value = map[string]int32{
	"DEBIT":  0,
	"CREDIT": 1,
}

func (x TransactionType) String() string {
	if x == TransactionType_CREDIT {
		return "CREDIT"
	}
	return "DEBIT"
}

type ListTransactionsRequest struct {
	PageSize   int32
	PageToken  string
	AccountId  string
	CategoryId []string
	Type       TransactionType
	DateFrom   time.Time
	DateTo     time.Time
	MinAmount  float32
	MaxAmount  float32
	Name       string
	NameMatch  string
	SortBy     string
	SortDir    string
	Offset     int32
}

type CreateTransactionsRequest struct {
	Transactions []*CreateTransactionRequest
}

type CreateTransactionRequest struct {
	Name        string
	Amount      float32
	Type        TransactionType
	OccurredAt  time.Time
	AccountId   string
	CategoryIds []string
	ExternalId  string
}

type UpdateTransactionsRequest struct {
	Transactions []*UpdateTransactionRequest
}

type UpdateTransactionRequest struct {
	Id          string
	Name        string
	Amount      float32
	Type        TransactionType
	OccurredAt  time.Time
	AccountId   string
	CategoryIds []string
}

type DeleteTransactionsRequest struct {
	Ids []string
}

type TransactionResponse struct {
	Id               string
	Name             string
	Amount           float32
	Type             TransactionType
	OccurredAt       time.Time
	AccountId        string
	CreatedAt        time.Time
	LinkedTransferId string
	CategoryIds      []string
	ExternalId       string
}

type ListTransactionsResponse struct {
	Transactions  []*TransactionResponse
	NextPageToken string
	TotalCount    int32
}

type BulkOperationResponse struct {
	Success   bool
	Message   string
	FailedIds []string
	Skipped   int32
}

type LinkTransfersRequest struct {
	Links []*LinkTransferRequest
}

type LinkTransferRequest struct {
	DebitTransactionId  string
	CreditTransactionId string
}

type UnlinkTransfersRequest struct {
	Ids []string
}

type CreateCounterpartRequest struct {
	TransactionId string
	ToAccountId   string
}

type CreateTransferResponse struct {
	DebitTransactionId  string
	CreditTransactionId string
}

// --- rules ---

type RuleLogic int32

const (
	RuleLogic_RULE_LOGIC_OR  RuleLogic = 1
	RuleLogic_RULE_LOGIC_AND RuleLogic = 2
)

var RuleLogic_value = map[string]int32{
	"RULE_LOGIC_OR":  1,
	"RULE_LOGIC_AND": 2,
}

func (x RuleLogic) String() string {
	if x == RuleLogic_RULE_LOGIC_AND {
		return "RULE_LOGIC_AND"
	}
	return "RULE_LOGIC_OR"
}

type RuleMatchField int32

const (
	RuleMatchField_RULE_MATCH_FIELD_UNSPECIFIED RuleMatchField = 0
	RuleMatchField_RULE_MATCH_FIELD_NAME        RuleMatchField = 1
	RuleMatchField_RULE_MATCH_FIELD_AMOUNT      RuleMatchField = 2
	RuleMatchField_RULE_MATCH_FIELD_TYPE        RuleMatchField = 3
	RuleMatchField_RULE_MATCH_FIELD_CATEGORY    RuleMatchField = 4
	RuleMatchField_RULE_MATCH_FIELD_ACCOUNT     RuleMatchField = 5
)

var RuleMatchField_value = map[string]int32{
	"RULE_MATCH_FIELD_UNSPECIFIED": 0,
	"RULE_MATCH_FIELD_NAME":        1,
	"RULE_MATCH_FIELD_AMOUNT":      2,
	"RULE_MATCH_FIELD_TYPE":        3,
	"RULE_MATCH_FIELD_CATEGORY":    4,
	"RULE_MATCH_FIELD_ACCOUNT":     5,
}

func (x RuleMatchField) String() string {
	return [...]string{
		"RULE_MATCH_FIELD_UNSPECIFIED",
		"RULE_MATCH_FIELD_NAME",
		"RULE_MATCH_FIELD_AMOUNT",
		"RULE_MATCH_FIELD_TYPE",
		"RULE_MATCH_FIELD_CATEGORY",
		"RULE_MATCH_FIELD_ACCOUNT",
	}[x]
}

type RuleMatchOperator int32

const (
	RuleMatchOperator_RULE_MATCH_OPERATOR_UNSPECIFIED  RuleMatchOperator = 0
	RuleMatchOperator_RULE_MATCH_OPERATOR_CONTAINS     RuleMatchOperator = 1
	RuleMatchOperator_RULE_MATCH_OPERATOR_STARTS_WITH  RuleMatchOperator = 2
	RuleMatchOperator_RULE_MATCH_OPERATOR_ENDS_WITH    RuleMatchOperator = 3
	RuleMatchOperator_RULE_MATCH_OPERATOR_EQUALS       RuleMatchOperator = 4
	RuleMatchOperator_RULE_MATCH_OPERATOR_GREATER_THAN RuleMatchOperator = 5
	RuleMatchOperator_RULE_MATCH_OPERATOR_LESS_THAN    RuleMatchOperator = 6
	RuleMatchOperator_RULE_MATCH_OPERATOR_REGEX        RuleMatchOperator = 7
)

var RuleMatchOperator_value = map[string]int32{
	"RULE_MATCH_OPERATOR_UNSPECIFIED":  0,
	"RULE_MATCH_OPERATOR_CONTAINS":     1,
	"RULE_MATCH_OPERATOR_STARTS_WITH":  2,
	"RULE_MATCH_OPERATOR_ENDS_WITH":    3,
	"RULE_MATCH_OPERATOR_EQUALS":       4,
	"RULE_MATCH_OPERATOR_GREATER_THAN": 5,
	"RULE_MATCH_OPERATOR_LESS_THAN":    6,
	"RULE_MATCH_OPERATOR_REGEX":        7,
}

func (x RuleMatchOperator) String() string {
	return [...]string{
		"RULE_MATCH_OPERATOR_UNSPECIFIED",
		"RULE_MATCH_OPERATOR_CONTAINS",
		"RULE_MATCH_OPERATOR_STARTS_WITH",
		"RULE_MATCH_OPERATOR_ENDS_WITH",
		"RULE_MATCH_OPERATOR_EQUALS",
		"RULE_MATCH_OPERATOR_GREATER_THAN",
		"RULE_MATCH_OPERATOR_LESS_THAN",
		"RULE_MATCH_OPERATOR_REGEX",
	}[x]
}

type RuleActionOp int32

const (
	RuleActionOp_RULE_ACTION_OP_UNSPECIFIED RuleActionOp = 0
	RuleActionOp_RULE_ACTION_OP_RENAME      RuleActionOp = 1
	RuleActionOp_RULE_ACTION_OP_ADD_PREFIX  RuleActionOp = 2
	RuleActionOp_RULE_ACTION_OP_ADD_SUFFIX  RuleActionOp = 3
)

var RuleActionOp_value = map[string]int32{
	"RULE_ACTION_OP_UNSPECIFIED": 0,
	"RULE_ACTION_OP_RENAME":      1,
	"RULE_ACTION_OP_ADD_PREFIX":  2,
	"RULE_ACTION_OP_ADD_SUFFIX":  3,
}

func (x RuleActionOp) String() string {
	return [...]string{
		"RULE_ACTION_OP_UNSPECIFIED",
		"RULE_ACTION_OP_RENAME",
		"RULE_ACTION_OP_ADD_PREFIX",
		"RULE_ACTION_OP_ADD_SUFFIX",
	}[x]
}

type ListRulesRequest struct{}

type CreateRuleRequest struct {
	Name       string
	Priority   int32
	Logic      RuleLogic
	Conditions []*RuleCondition
	Actions    []*RuleAction
}

type UpdateRuleRequest struct {
	Id         string
	Name       string
	Priority   int32
	Logic      RuleLogic
	Conditions []*RuleCondition
	Actions    []*RuleAction
}

type DeleteRuleRequest struct {
	Id string
}

type RuleCondition struct {
	MatchField RuleMatchField
	Operator   RuleMatchOperator
	Pattern    string
}

type RuleAction struct {
	SetName              string
	SetNameOp            RuleActionOp
	SetCategoryId        string
	SetTransferAccountId string
}

type RuleResponse struct {
	Id         string
	Name       string
	Priority   int32
	Logic      RuleLogic
	Conditions []*RuleCondition
	Actions    []*RuleAction
	CreatedAt  time.Time
}

type ListRulesResponse struct {
	Rules []*RuleResponse
}

type PreviewRuleRequest struct {
	Logic      RuleLogic
	Conditions []*RuleCondition
	Limit      int32
}

type RunRuleResponse struct {
	Matched int32
	Linked  int32
	Created int32
}

// --- investments ---

type InvestmentType int32

const (
	InvestmentType_INVESTMENT_TYPE_UNSPECIFIED InvestmentType = 0
	InvestmentType_INVESTMENT_TYPE_STOCK       InvestmentType = 1
	InvestmentType_INVESTMENT_TYPE_MUTUAL_FUND InvestmentType = 2
)

var InvestmentType_value = map[string]int32{
	"INVESTMENT_TYPE_UNSPECIFIED": 0,
	"INVESTMENT_TYPE_STOCK":       1,
	"INVESTMENT_TYPE_MUTUAL_FUND": 2,
}

func (x InvestmentType) String() string {
	switch x {
	case InvestmentType_INVESTMENT_TYPE_STOCK:
		return "INVESTMENT_TYPE_STOCK"
	case InvestmentType_INVESTMENT_TYPE_MUTUAL_FUND:
		return "INVESTMENT_TYPE_MUTUAL_FUND"
	default:
		return "INVESTMENT_TYPE_UNSPECIFIED"
	}
}

type InvestmentResponse struct {
	Id             string
	Symbol         string
	Name           string
	InvestmentType InvestmentType
	CurrentPrice   float32
	PrevClose      float32
	ManualNav      float32
	LastQuoteAt    time.Time
	CreatedAt      time.Time
	Quantity       float32
	AvgCost        float32
	CurrentValue   float32
	UnrealizedPnl  float32
	RealizedPnl    float32
}

type LotResponse struct {
	Id           string
	InvestmentId string
	Side         int32
	Quantity     float32
	Price        float32
	OccurredAt   time.Time
	CreatedAt    time.Time
}

type CreateInvestmentRequest struct {
	Symbol         string
	Name           string
	InvestmentType InvestmentType
	ManualNav      float32
}

type UpdateInvestmentRequest struct {
	Id             string
	Name           string
	InvestmentType InvestmentType
	ManualNav      float32
}

type DeleteInvestmentRequest struct{ Id string }

type GetInvestmentRequest struct{ Id string }

type ListInvestmentsRequest struct{}

type ListInvestmentsResponse struct {
	Investments []*InvestmentResponse
}

type AddLotRequest struct {
	InvestmentId string
	Side         int32
	Quantity     float32
	Price        float32
	OccurredAt   time.Time
}

type DeleteLotRequest struct{ Id string }

type RefreshPricesRequest struct{}

type RefreshPricesResponse struct {
	Updated int32
}

type SearchSymbolsRequest struct{ Query string }

type SymbolResult struct {
	Symbol         string
	Name           string
	InvestmentType InvestmentType
}

type SearchSymbolsResponse struct {
	Results []*SymbolResult
}

type GetPortfolioSummaryRequest struct{}

type PortfolioSummaryResponse struct {
	TotalInvested      float32
	TotalCurrentValue  float32
	TotalUnrealizedPnl float32
	TotalRealizedPnl   float32
}

type DashboardResponse struct {
	TotalBalance   float32
	TotalIncome    float32
	TotalExpenses  float32
	PortfolioValue float32
	Accounts       []*AccountResponse
	Investments    []*InvestmentResponse
}

type SpendingRequest struct {
	Range     string
	From      string
	To        string
	AccountId string
}

type SpendingBucket struct {
	Key    string
	Label  string
	Amount float32
}

type SpendingCategory struct {
	Id     string
	Name   string
	Debit  float32
	Credit float32
	Net    float32
}

type SpendingResponse struct {
	Buckets    []*SpendingBucket
	Categories []*SpendingCategory
}
