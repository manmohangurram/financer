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

type GetMeRequest struct{}

type AuthResponse struct {
	AccessToken  string
	RefreshToken string
	UserId       string
	Email        string
	Name         string
}

type GetMeResponse struct {
	UserId string
	Email  string
	Name   string
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

type GetTransactionRequest struct {
	Id string
}

type ListTransactionsRequest struct {
	PageSize   int32
	PageToken  string
	AccountId  string
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
	Name       string
	Amount     float32
	Type       TransactionType
	OccurredAt time.Time
	AccountId  string
}

type UpdateTransactionsRequest struct {
	Transactions []*UpdateTransactionRequest
}

type UpdateTransactionRequest struct {
	Id         string
	Name       string
	Amount     float32
	Type       TransactionType
	OccurredAt time.Time
	AccountId  string
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

type TransferLinkResponse struct {
	Id                  string
	DebitTransactionId  string
	CreditTransactionId string
	CreatedAt           time.Time
}
