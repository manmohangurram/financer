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

// MailTemplatePatterns is a manual extraction template: each field holds one or
// more RE2 patterns tried in order, and the first that matches wins.
type MailTemplatePatterns struct {
	Amount   []string `json:"amount"`
	Debit    []string `json:"debit,omitempty"`
	Credit   []string `json:"credit,omitempty"`
	Account  []string `json:"account,omitempty"`
	Merchant []string `json:"merchant,omitempty"`
	Date     []string `json:"date"`
}

// ReqMailTemplateValidate tests template patterns against a pasted sample mail.
type ReqMailTemplateValidate struct {
	Subject  string               `json:"subject"`
	Body     string               `json:"body"`
	Patterns MailTemplatePatterns `json:"patterns"`
}

// MailTemplatePatternCheck is the result of one pattern against the sample.
type MailTemplatePatternCheck struct {
	Pattern string `json:"pattern"`
	Matched bool   `json:"matched"`
	Value   string `json:"value,omitempty"`
	Error   string `json:"error,omitempty"`
}

// MailTemplateFieldCheck groups the pattern results for one field.
type MailTemplateFieldCheck struct {
	Field    string                     `json:"field"`
	Patterns []MailTemplatePatternCheck `json:"patterns"`
}

// MailTemplateValidation is the editor's preview of one extraction.
type MailTemplateValidation struct {
	Fields    []MailTemplateFieldCheck `json:"fields"`
	Amount    float64                  `json:"amount"`
	Direction string                   `json:"direction,omitempty"`
	Account   string                   `json:"account,omitempty"`
	Merchant  string                   `json:"merchant,omitempty"`
	Date      string                   `json:"date,omitempty"`
	Complete  bool                     `json:"complete"`
	Reason    string                   `json:"reason,omitempty"`
}

// MailRule is a disposition rule.
type MailRule struct {
	ID              string                `json:"id"`
	Position        int64                 `json:"position"`
	Enabled         bool                  `json:"enabled"`
	FromPattern     string                `json:"fromPattern"`
	SubjectContains string                `json:"subjectContains"`
	BodyContains    string                `json:"bodyContains"`
	Action          string                `json:"action"`
	AccountID       *string               `json:"accountId"`
	CategoryIDs     []string              `json:"categoryIds"`
	Template        *MailTemplatePatterns `json:"template,omitempty"`
}

// ReqMailRule is the create/update rule request.
type ReqMailRule struct {
	Position        int64                 `json:"position"`
	Enabled         *bool                 `json:"enabled"`
	FromPattern     string                `json:"fromPattern"`
	SubjectContains string                `json:"subjectContains"`
	BodyContains    string                `json:"bodyContains"`
	Action          string                `json:"action"`
	AccountID       *string               `json:"accountId"`
	CategoryIDs     []string              `json:"categoryIds"`
	Template        *MailTemplatePatterns `json:"template,omitempty"`
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
	Accounts      []*MailAccount `json:"accounts"`
	NextPageToken string         `json:"nextPageToken"`
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
	Rules         []*MailRule `json:"rules"`
	NextPageToken string      `json:"nextPageToken"`
}

// AIParseStatus is the progress of the per-sender AI re-parse job.
type AIParseStatus struct {
	Running       bool   `json:"running"`
	TotalSenders  int64  `json:"totalSenders"`
	DoneSenders   int64  `json:"doneSenders"`
	CurrentSender string `json:"currentSender"`
	StartedAt     string `json:"startedAt"`
	FinishedAt    string `json:"finishedAt"`
	Error         string `json:"error"`
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
