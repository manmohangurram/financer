package mcpserver

import (
	"strings"

	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/transactionservice"
)

type listTransactionsReq struct {
	CategoryIDs []string `json:"categoryIds,omitempty"`
	Ty          *string  `json:"ty,omitempty"`
}

type ruleReq struct {
	Name       string                `json:"name"`
	Priority   int64                 `json:"priority,omitempty"`
	Logic      string                `json:"logic,omitempty"`
	Conditions []model.RuleCondition `json:"conditions,omitempty"`
	Actions    []model.RuleAction    `json:"actions,omitempty"`
}

type updateRuleReq struct {
	ID         string                `json:"id"`
	Name       string                `json:"name,omitempty"`
	Priority   int64                 `json:"priority,omitempty"`
	Logic      string                `json:"logic,omitempty"`
	Conditions []model.RuleCondition `json:"conditions,omitempty"`
	Actions    []model.RuleAction    `json:"actions,omitempty"`
}

type createCategoryReq struct {
	Names []string `json:"names"`
}

type catItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type categoriesReq struct {
	IDs        []string  `json:"ids,omitempty"`
	Categories []catItem `json:"categories,omitempty"`
}

type deleteReq struct {
	ID string `json:"id"`
}

type txnPayload struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name"`
	Amount      float64  `json:"amount"`
	Type        string   `json:"type"`
	OccurredAt  string   `json:"occurredAt,omitempty"`
	AccountID   string   `json:"accountId,omitempty"`
	CategoryIDs []string `json:"categoryIds,omitempty"`
	ExternalID  *string  `json:"externalId,omitempty"`
	RefNo       string   `json:"refNo,omitempty"`
}

type createTransactionsReq struct {
	Transactions []txnPayload `json:"transactions"`
}

type updateTransactionsReq struct {
	Transactions []txnPayload `json:"transactions"`
}

type deleteTransactionsReq struct {
	IDs []string `json:"ids"`
}

type createAccountReq struct {
	BankName      string `json:"bankName"`
	Nickname      string `json:"nickname,omitempty"`
	EndingNumbers string `json:"endingNumbers"`
	Type          string `json:"type"`
}

type updateAccountReq struct {
	ID            string `json:"id"`
	BankName      string `json:"bankName,omitempty"`
	Nickname      string `json:"nickname,omitempty"`
	EndingNumbers string `json:"endingNumbers,omitempty"`
	Type          string `json:"type"`
}

type transferLink struct {
	DebitTransactionID  string `json:"debitTransactionId,omitempty"`
	CreditTransactionID string `json:"creditTransactionId,omitempty"`
}

type transfersReq struct {
	IDs   []string       `json:"ids,omitempty"`
	Links []transferLink `json:"links,omitempty"`
}

type createTransferCounterpartReq struct {
	TransactionID string `json:"transactionId,omitempty"`
	ToAccountID   string `json:"toAccountId,omitempty"`
}

type previewRuleReq struct {
	Logic      string                `json:"logic,omitempty"`
	Conditions []model.RuleCondition `json:"conditions,omitempty"`
	Limit      int64                 `json:"limit,omitempty"`
}

type runRuleReq struct {
	ID string `json:"id"`
}

func (p txnPayload) toRequest() (transactionservice.Request, error) {
	typ := strings.ToUpper(p.Type)
	if typ != string(model.TransactionTypeDebit) && typ != string(model.TransactionTypeCredit) {
		return transactionservice.Request{}, errors.BadRequest("unknown transaction type " + p.Type)
	}
	return transactionservice.Request{
		ID:          p.ID,
		Name:        p.Name,
		Amount:      p.Amount,
		Type:        typ,
		OccurredAt:  normalizeOccurredAt(p.OccurredAt),
		AccountID:   p.AccountID,
		CategoryIDs: p.CategoryIDs,
		ExternalID:  p.ExternalID,
		RefNo:       p.RefNo,
	}, nil
}

func validAccountType(v string) bool {
	switch strings.ToUpper(v) {
	case "CURRENT", "SAVINGS", "LOAN", "CREDIT_CARD":
		return true
	}
	return false
}
