package repo

// MailStatus is the disposition of a fetched message.
type MailStatus string

const (
	MailStatusPending  MailStatus = "PENDING"
	MailStatusUnparsed MailStatus = "UNPARSED"
	MailStatusIgnored  MailStatus = "IGNORED"
	MailStatusAuto     MailStatus = "AUTO"
	MailStatusRejected MailStatus = "REJECTED"
)

// MailRuleAction is what a matching mail rule does with a message.
type MailRuleAction string

const (
	MailRuleActionIgnore MailRuleAction = "IGNORE"
	MailRuleActionReview MailRuleAction = "REVIEW"
	MailRuleActionAuto   MailRuleAction = "AUTO"
)

// MailAccountRow is a linked mailbox as stored.
type MailAccountRow struct {
	ID              string
	UserID          string
	Label           string
	IMAPHost        string
	IMAPPort        int64
	Username        string
	SecretEnc       string
	SenderAllowlist []string
	Enabled         bool
	LastPolledAt    *string
	LastError       string
	CreatedAt       string
}

// NewMailAccount is the input for linking a mailbox.
type NewMailAccount struct {
	Label           string
	IMAPHost        string
	IMAPPort        int64
	Username        string
	SecretEnc       string
	SenderAllowlist []string
}

// NewMailMessage is a message about to be stored; MessageID is the
// idempotency key.
type NewMailMessage struct {
	MailAccountID   string
	MessageID       string
	FromAddr        string
	ReceivedAt      *string
	Subject         string
	RawText         string
	Status          MailStatus
	ParsedJSON      *string
	Confidence      float64
	ExtractorSource string
}

// MailMessageRow is a stored message as the review UI sees it.
type MailMessageRow struct {
	ID            string
	MessageID     string
	FromAddr      string
	Subject       string
	ReceivedAt    *string
	Status        MailStatus
	ParsedJSON    *string
	Confidence    float64
	RawText       *string
	TransactionID *string
}

// MailRuleRow is a disposition rule.
type MailRuleRow struct {
	ID              string
	Position        int64
	Enabled         bool
	FromPattern     string
	SubjectContains string
	BodyContains    string
	Action          MailRuleAction
	AccountID       *string
	CategoryIDs     []string
}

// MailRuleInput is a create/update rule payload.
type MailRuleInput struct {
	Position        int64
	Enabled         bool
	FromPattern     string
	SubjectContains string
	BodyContains    string
	Action          MailRuleAction
	AccountID       *string
	CategoryIDs     []string
}

// MailRepo is the backend-agnostic mail-ingest repository.
type MailRepo interface {
	CreateAccount(userID string, input *NewMailAccount) (*MailAccountRow, error)
	ListAccounts(userID string) ([]*MailAccountRow, error)
	DeleteAccount(userID, id string) (bool, error)
	ListPollableAccounts() ([]*MailAccountRow, error)
	MarkPolled(accountID, errMsg string) error
	KnownMessageIDs(userID string, ids []string) ([]string, error)
	InsertMessage(userID string, msg *NewMailMessage) (bool, error)
	ListMessages(userID string, status *MailStatus, limit int64) ([]*MailMessageRow, error)
	GetMessage(userID, id string) (*MailMessageRow, error)
	ResolveMessage(userID, id string, status MailStatus, approvedJSON, transactionID *string) (bool, error)
	ListRules(userID string) ([]*MailRuleRow, error)
	CreateRule(userID string, input *MailRuleInput) (*MailRuleRow, error)
	UpdateRule(userID, id string, input *MailRuleInput) (bool, error)
	DeleteRule(userID, id string) (bool, error)
	SweepBodies() (int64, error)
}
