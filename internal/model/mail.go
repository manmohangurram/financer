package model

// MailAccount is a linked mailbox as returned to the UI (never the secret).
type MailAccount struct {
	ID           string  `json:"id"`
	Label        string  `json:"label"`
	IMAPHost     string  `json:"imapHost"`
	IMAPPort     int64   `json:"imapPort"`
	Username     string  `json:"username"`
	Enabled      bool    `json:"enabled"`
	LastPolledAt *string `json:"lastPolledAt"`
	LastError    string  `json:"lastError"`
}

// ReqMailAccount is the link-mailbox request.
type ReqMailAccount struct {
	Label    string `json:"label"`
	IMAPHost string `json:"imapHost"`
	IMAPPort int64  `json:"imapPort"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// ReqUpdateMailAccount edits a mailbox; an empty password keeps the stored one.
type ReqUpdateMailAccount struct {
	Label    string `json:"label"`
	IMAPHost string `json:"imapHost"`
	IMAPPort int64  `json:"imapPort"`
	Username string `json:"username"`
	Password string `json:"password"`
	Enabled  *bool  `json:"enabled"`
}

// MailMessage is a stored message as the review UI sees it.
type MailMessage struct {
	ID            string  `json:"id"`
	MessageID     string  `json:"messageId"`
	MailAccountID string  `json:"mailAccountId"`
	FromAddr      string  `json:"fromAddr"`
	FromName      string  `json:"fromName"`
	Subject       string  `json:"subject"`
	ReceivedAt    *string `json:"receivedAt"`
	Status        string  `json:"status"`
	ParsedJSON    *string `json:"parsedJson"`
	Confidence    float64 `json:"confidence"`
	BodyPreview   string  `json:"bodyPreview"`
	BodyText      *string `json:"bodyText,omitempty"`
	RawText       *string `json:"rawText,omitempty"`
	TransactionID *string `json:"transactionId"`
}

// MailRule is a disposition rule.
type MailRule struct {
	ID              string   `json:"id"`
	Position        int64    `json:"position"`
	Enabled         bool     `json:"enabled"`
	FromPattern     string   `json:"fromPattern"`
	SubjectContains string   `json:"subjectContains"`
	BodyContains    string   `json:"bodyContains"`
	Action          string   `json:"action"`
	AccountID       *string  `json:"accountId"`
	CategoryIDs     []string `json:"categoryIds"`
}

// ReqMailRule is the create/update rule request.
type ReqMailRule struct {
	Position        int64    `json:"position"`
	Enabled         *bool    `json:"enabled"`
	FromPattern     string   `json:"fromPattern"`
	SubjectContains string   `json:"subjectContains"`
	BodyContains    string   `json:"bodyContains"`
	Action          string   `json:"action"`
	AccountID       *string  `json:"accountId"`
	CategoryIDs     []string `json:"categoryIds"`
}

// ReqMailAction is the approve/reject batch request.
type ReqMailAction struct {
	Messages []MailActionItem `json:"messages"`
}

// MailActionItem is one row decision, with optional field overrides that replace
// the extracted values before the transaction is created.
type MailActionItem struct {
	ID          string   `json:"id"`
	Action      string   `json:"action"` // APPROVE / REJECT
	AccountID   *string  `json:"accountId"`
	CategoryIDs []string `json:"categoryIds"`
	Name        *string  `json:"name"`
	Amount      *float64 `json:"amount"`
	Type        *string  `json:"type"`
	OccurredAt  *string  `json:"occurredAt"`
}

// MailActionFailure reports one failed row.
type MailActionFailure struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

// MailActionOutcome is the per-row result of an action batch.
type MailActionOutcome struct {
	Succeeded []string            `json:"succeeded"`
	Failed    []MailActionFailure `json:"failed"`
}

// WireMailAccounts is the GET /api/mail/accounts response.
type WireMailAccounts struct {
	Accounts []*MailAccount `json:"accounts"`
}

// WireMailMessages is the GET /api/mail/messages response.
type WireMailMessages struct {
	Messages      []*MailMessage `json:"messages"`
	Total         int64          `json:"total"`
	TotalCount    int64          `json:"totalCount"`
	NextPageToken string         `json:"nextPageToken"`
}

// WireMailRules is the GET /api/mail/rules response.
type WireMailRules struct {
	Rules []*MailRule `json:"rules"`
}

// WirePollStatus is the async poll trigger response.
type WirePollStatus struct {
	Status string `json:"status"` // started | throttled
}

// WirePollSummary is the POST /api/mail/accounts/{id}/poll response.
type WirePollSummary struct {
	Matched int `json:"matched"`
	Stored  int `json:"stored"`
	Skipped int `json:"skipped"`
}
