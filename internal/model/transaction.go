package model

// TransactionType is the stored/wire transaction direction.
type TransactionType string

const (
	TransactionTypeDebit  TransactionType = "DEBIT"
	TransactionTypeCredit TransactionType = "CREDIT"
)

// ReqTxn is the create/update/delete bulk envelope.
type ReqTxn struct {
	Transactions []ReqListTxn `json:"transactions"`
	IDs          []string     `json:"ids"`
}

// ReqListTxn is one transaction in a create/update request. OccurredAt is raw
// JSON: {seconds,nanos}, RFC3339, or YYYY-MM-DD.
type ReqListTxn struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Amount      float64         `json:"amount"`
	Type        TransactionType `json:"type"`
	OccurredAt  jsonRaw         `json:"occurredAt"`
	AccountID   string          `json:"accountId"`
	CategoryIDs []string        `json:"categoryIds"`
	ExternalID  string          `json:"externalId"`
	RefNo       string          `json:"refNo"`
}

// WireTxn is one transaction in a list response.
type WireTxn struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Amount           float64   `json:"amount"`
	Type             string    `json:"type"`
	OccurredAt       string    `json:"occurredAt"`
	AccountID        string    `json:"accountId"`
	CreatedAt        string    `json:"createdAt"`
	LinkedTransferID string    `json:"linkedTransferId"`
	CategoryIDs      *[]string `json:"categoryIds"`
	RefNo            string    `json:"refNo"`
}

// WireBulk is the bulk-operation result; empty failedIds and zero skipped are
// omitted (matching Rust's skip_serializing_if).
type WireBulk struct {
	Success   bool     `json:"success"`
	Message   string   `json:"message"`
	FailedIDs []string `json:"failedIds,omitempty"`
	Skipped   int64    `json:"skipped,omitempty"`
}

// WireTxnList is the GET /api/transactions response.
type WireTxnList struct {
	Transactions  []WireTxn `json:"transactions"`
	NextPageToken string    `json:"nextPageToken"`
	TotalCount    int64     `json:"totalCount"`
}

// WireTxnPreview is one transaction in a rules-preview response.
type WireTxnPreview struct {
	Name        string   `json:"name"`
	Amount      float64  `json:"amount"`
	Type        string   `json:"type"`
	AccountID   string   `json:"accountId"`
	CategoryIDs []string `json:"categoryIds"`
	OccurredAt  string   `json:"occurredAt"`
}

// WireDashboard is the GET /api/dashboard response.
type WireDashboard struct {
	TotalBalance  float64            `json:"totalBalance"`
	TotalIncome   float64            `json:"totalIncome"`
	TotalExpenses float64            `json:"totalExpenses"`
	Accounts      []*AccountResponse `json:"accounts"`
}

// WireSpendingBucket is one day/month spending bucket.
type WireSpendingBucket struct {
	Key    string  `json:"key"`
	Label  string  `json:"label"`
	Amount float64 `json:"amount"`
}

// WireSpendingCategory is the debit/credit aggregate for one category.
type WireSpendingCategory struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Debit  float64 `json:"debit"`
	Credit float64 `json:"credit"`
	Net    float64 `json:"net"`
}

// WireSpending is the GET /api/spending response.
type WireSpending struct {
	Buckets    []WireSpendingBucket   `json:"buckets"`
	Categories []WireSpendingCategory `json:"categories"`
}

// ReqDuplicateCheck is the pre-import duplicate probe.
type ReqDuplicateCheck struct {
	Transactions []ReqListTxn `json:"transactions"`
}

// WireDuplicates marks which candidate rows already exist, by index.
type WireDuplicates struct {
	Duplicates []bool `json:"duplicates"`
}
