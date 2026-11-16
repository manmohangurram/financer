package model

// ReqTransfer is the transfer-link create/delete envelope.
type ReqTransfer struct {
	IDs   []string  `json:"ids"`
	Links []ReqLink `json:"links"`
}

// ReqLink is one debit/credit transaction pair to link.
type ReqLink struct {
	DebitTransactionID  string `json:"debitTransactionId"`
	CreditTransactionID string `json:"creditTransactionId"`
}

// ReqCreateCounterpart asks to create/link the missing side of a transfer.
type ReqCreateCounterpart struct {
	TransactionID string `json:"transactionId"`
	ToAccountID   string `json:"toAccountId"`
}

// CreateTransferResp is the counterpart creation response.
type CreateTransferResp struct {
	DebitTransactionID  string `json:"debitTransactionId"`
	CreditTransactionID string `json:"creditTransactionId"`
}
