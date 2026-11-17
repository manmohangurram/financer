package repo

// TransactionRow is a transaction as stored/returned by the repository layer.
type TransactionRow struct {
	ID             string
	Name           string
	CleanName      *string
	Amount         float64
	Type           string // DEBIT / CREDIT
	OccurredAt     string
	AccountID      string
	CreatedAt      string
	ExternalID     *string
	RefNo          string
	TransferLinked bool
}

// CreateTransactionInput is one transaction to create plus its category links.
type CreateTransactionInput struct {
	Txn         *TransactionRow
	CategoryIDs []string
}

// TransactionKey identifies a stored transaction for duplicate checks.
type TransactionKey struct {
	Date   string
	Type   string
	Amount float64
}

// UpdateTransactionInput is one transaction to update plus its full category set.
type UpdateTransactionInput struct {
	Txn         *TransactionRow
	CategoryIDs []string
}

// TransactionListFilter mirrors the Rust service filter.
type TransactionListFilter struct {
	AccountID   string
	CategoryIDs []string
	Type        *string
	DateFrom    string
	DateTo      string
	MinAmount   float64
	MaxAmount   float64
	Names       []string
	PageSize    int64
	PageToken   string
	SortBy      string
	SortDir     string
	Offset      int64
}

// ListRow is a listed transaction plus its transfer link id and category ids.
type ListRow struct {
	Txn         *TransactionRow
	LinkID      string
	CategoryIDs []string
}

// ListTransactionResult is one page of transactions.
type ListTransactionResult struct {
	Rows          []ListRow
	NextPageToken string
	TotalCount    int64
}

// SpendingFilter scopes spending aggregation rows.
type SpendingFilter struct {
	From      string
	To        string
	AccountID string
}

// SpendingTxn is one spending-relevant transaction (one row per category).
type SpendingTxn struct {
	ID           string
	OccurredAt   string
	Amount       float64
	Type         string
	CategoryID   *string
	CategoryName *string
}

// CreateOutcome reports which transaction ids were inserted and any per-row errors.
type CreateOutcome struct {
	Inserted map[string]bool
	Errors   []string
}

// TransactionView is a transaction's matchable view used by the rule overlay.
type TransactionView struct {
	Name        string
	Amount      float64
	Type        string
	AccountID   string
	CategoryIDs []string
	OccurredAt  string
}

// TransactionRepo is the backend-agnostic transaction/transfer-link repository.
type TransactionRepo interface {
	GetForTransfer(userID, id string) (typ string, amount float64, accountID string, ok bool, err error)
	GetFull(userID, id string) (*TransactionRow, error)
	FindTransferCounterpart(userID, accountID, typ string, amount float64, around string) (*TransactionRow, error)
	IsTransferLinked(userID, txnID string) (bool, error)
	// ExistingTransactionKeys returns the (date, type, amount) keys already stored
	// for an account between two YYYY-MM-DD dates, for duplicate detection.
	ExistingTransactionKeys(userID, accountID, from, to string) ([]TransactionKey, error)
	Create(userID string, inputs []CreateTransactionInput) (*CreateOutcome, error)
	Update(userID string, inputs []UpdateTransactionInput) ([]string, error)
	ApplyRuleResult(userID, id string, cleanName *string, categoryIDs []string) error
	Delete(userID string, ids []string) ([]string, error)
	List(userID string, f *TransactionListFilter) (*ListTransactionResult, error)
	SpendingRows(userID string, f *SpendingFilter) ([]*SpendingTxn, error)
	CreateLinks(userID string, links [][2]string) ([]string, error)
	DeleteLinks(userID string, ids []string) ([]string, error)
}
