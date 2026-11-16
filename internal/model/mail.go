package model

// MailAccount is a linked mailbox as returned to the UI (never the secret).
type MailAccount struct {
	ID              string   `json:"id"`
	Label           string   `json:"label"`
	IMAPHost        string   `json:"imapHost"`
	IMAPPort        int64    `json:"imapPort"`
	Username        string   `json:"username"`
	SenderAllowlist []string `json:"senderAllowlist"`
	Enabled         bool     `json:"enabled"`
	LastPolledAt    *string  `json:"lastPolledAt"`
	LastError       string   `json:"lastError"`
}

// ReqMailAccount is the link-mailbox request.
type ReqMailAccount struct {
	Label           string   `json:"label"`
	IMAPHost        string   `json:"imapHost"`
	IMAPPort        int64    `json:"imapPort"`
	Username        string   `json:"username"`
	Password        string   `json:"password"`
	SenderAllowlist []string `json:"senderAllowlist"`
}

// MailMessage is a stored message as the review UI sees it.
type MailMessage struct {
	ID            string  `json:"id"`
	MessageID     string  `json:"messageId"`
	FromAddr      string  `json:"fromAddr"`
	Subject       string  `json:"subject"`
	ReceivedAt    *string `json:"receivedAt"`
	Status        string  `json:"status"`
	ParsedJSON    *string `json:"parsedJson"`
	Confidence    float64 `json:"confidence"`
	RawText       *string `json:"rawText"`
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

// MailActionItem is one row decision.
type MailActionItem struct {
	ID          string   `json:"id"`
	Action      string   `json:"action"` // APPROVE / REJECT
	AccountID   *string  `json:"accountId"`
	CategoryIDs []string `json:"categoryIds"`
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
	Messages []*MailMessage `json:"messages"`
}

// WireMailRules is the GET /api/mail/rules response.
type WireMailRules struct {
	Rules []*MailRule `json:"rules"`
}

// WirePollSummary is the POST /api/mail/accounts/{id}/poll response.
type WirePollSummary struct {
	Matched int `json:"matched"`
	Stored  int `json:"stored"`
	Skipped int `json:"skipped"`
}
