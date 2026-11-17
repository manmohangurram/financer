// Package mcpserver exposes Financer data over the Model Context Protocol at
// /mcp, authenticated by a per-user API key. Every server instance is bound to
// one authenticated user and key scope (read or read_write).
package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/mohan9182/financer/internal/accountservice"
	"github.com/mohan9182/financer/internal/categoryservice"
	"github.com/mohan9182/financer/internal/mailservice"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo"
	"github.com/mohan9182/financer/internal/ruleservice"
	tim "github.com/mohan9182/financer/internal/time"
	"github.com/mohan9182/financer/internal/transactionservice"
	"github.com/mohan9182/financer/internal/transferservice"
)

// Services is the set of domain services the tools call.
type Services struct {
	Accounts      *accountservice.Service
	Transactions  *transactionservice.Service
	Categories    *categoryservice.Service
	Rules         *ruleservice.Service
	Transfers     *transferservice.Service
	TransferRules *ruleservice.TransferRuleService
	Mail          *mailservice.Service
}

type handler struct {
	svc   Services
	uid   string
	scope string
}

func (h *handler) canWrite() bool {
	return strings.EqualFold(h.scope, "read_write")
}

// Build returns an MCP server whose tools act as the given user.
func Build(svc Services, uid, scope string) *mcp.Server {
	h := &handler{svc: svc, uid: uid, scope: scope}
	s := mcp.NewServer(&mcp.Implementation{Name: "financer", Version: "1.0.0"}, nil)

	// --- read tools ---
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_accounts",
		Description: "List the user's accounts with balances. Example: no arguments.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		rows, err := h.svc.Accounts.List(h.uid)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(rows), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_transactions",
		Description: `List the user's transactions. Example args: {"categoryIds":["<id>"], "ty":"DEBIT"}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listTransactionsReq) (*mcp.CallToolResult, any, error) {
		f := &repo.TransactionListFilter{CategoryIDs: in.CategoryIDs}
		if in.Ty != nil {
			t := strings.ToUpper(*in.Ty)
			f.Type = &t
		}
		res, err := h.svc.Transactions.List(h.uid, f)
		if err != nil {
			return errResult(err), nil, nil
		}
		items := make([]map[string]any, 0, len(res.Rows))
		for _, row := range res.Rows {
			items = append(items, map[string]any{
				"id":             row.Txn.ID,
				"name":           row.Txn.Name,
				"cleanName":      row.Txn.CleanName,
				"amount":         row.Txn.Amount,
				"type":           row.Txn.Type,
				"occurredAt":     tim.TSRFC3339(row.Txn.OccurredAt),
				"accountId":      row.Txn.AccountID,
				"createdAt":      tim.TSRFC3339(row.Txn.CreatedAt),
				"externalId":     row.Txn.ExternalID,
				"transferLinked": row.LinkID != "",
				"linkId":         row.LinkID,
				"categoryIds":    row.CategoryIDs,
			})
		}
		return okResult(items), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_balance_summary",
		Description: "Total balance and credit/debit totals across accounts. Example: no arguments.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		dash, err := h.svc.Transactions.Dashboard(h.uid)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(map[string]any{
			"totalBalance":  dash.TotalBalance,
			"totalIncome":   dash.TotalIncome,
			"totalExpenses": dash.TotalExpenses,
		}), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_categories",
		Description: "List the user's categories. Example: no arguments.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		rows, _, err := h.svc.Categories.List(h.uid, 100, "")
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(rows), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_rules",
		Description: "List the user's rules. Example: no arguments.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		rows, err := h.svc.Rules.List(h.uid)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(rows), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "preview_rule",
		Description: `Preview which transactions match a rule (no mutation). Example args: {"logic":"AND","conditions":[{"matchField":"NAME","operator":"CONTAINS","pattern":"swiggy"}]}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in previewRuleReq) (*mcp.CallToolResult, any, error) {
		views, err := h.svc.Rules.Preview(h.uid, in.Logic, in.Conditions, in.Limit)
		if err != nil {
			return errResult(err), nil, nil
		}
		items := make([]map[string]any, 0, len(views))
		for _, v := range views {
			items = append(items, map[string]any{
				"name": v.Name, "amount": v.Amount, "type": v.Type,
				"accountId": v.AccountID, "categoryIds": v.CategoryIDs,
			})
		}
		return okResult(map[string]any{"transactions": items}), nil, nil
	})

	// --- write tools ---
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_category",
		Description: `Create categories by name. Example args: {"names":["Food Delivery"]}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createCategoryReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot create categories"), nil, nil
		}
		return okResult(h.svc.Categories.Create(h.uid, categoryInputs(in.Names))), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_category",
		Description: `Update categories by id+name. Example args: {"categories":[{"id":"<id>","name":"New"}]}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in categoriesReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot update categories"), nil, nil
		}
		inputs := make([]repo.CategoryUpdateInput, 0, len(in.Categories))
		for _, c := range in.Categories {
			inputs = append(inputs, repo.CategoryUpdateInput{ID: c.ID, Name: c.Name})
		}
		return okResult(h.svc.Categories.Update(h.uid, inputs)), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_category",
		Description: `Delete categories by id. Example args: {"ids":["<id>"]}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in categoriesReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot delete categories"), nil, nil
		}
		return okResult(h.svc.Categories.Delete(h.uid, in.IDs)), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_rule",
		Description: `Create a rule. Example args: {"name":"Swiggy","conditions":[{"matchField":"NAME","operator":"CONTAINS","pattern":"swiggy"}],"actions":[{"setName":"Swiggy"}]}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in ruleReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot create rules"), nil, nil
		}
		rule, err := h.svc.Rules.Create(h.uid, in.Name, in.Priority, logicOf(in.Logic), in.Conditions, in.Actions)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(rule), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_rule",
		Description: `Update a rule by id. Example args: {"id":"<rule-id>","name":"Swiggy"}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in updateRuleReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot update rules"), nil, nil
		}
		rule, err := h.svc.Rules.Update(h.uid, in.ID, in.Name, in.Priority, logicOf(in.Logic), in.Conditions, in.Actions)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(rule), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_rule",
		Description: `Delete a rule by id. Example args: {"id":"<rule-id>"}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in deleteReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot delete rules"), nil, nil
		}
		if err := h.svc.Rules.Delete(h.uid, in.ID); err != nil {
			return errResult(err), nil, nil
		}
		return okResult(map[string]bool{"ok": true}), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "run_rule",
		Description: `Run a rule against existing transactions (applies it). Example args: {"id":"<rule-id>"}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in runRuleReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot run rules"), nil, nil
		}
		if _, err := h.svc.Rules.Run(h.uid, in.ID); err != nil {
			return errResult(err), nil, nil
		}
		tr, err := h.svc.TransferRules.RunRule(h.uid, in.ID)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(map[string]any{
			"matched": tr.Matched, "linked": tr.Linked, "created": tr.Created,
		}), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_transaction",
		Description: `Create transactions. Example args: {"transactions":[{"name":"Coffee","amount":4.5,"type":"DEBIT","accountId":"<id>"}]}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createTransactionsReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot create transactions"), nil, nil
		}
		reqs := make([]transactionservice.Request, 0, len(in.Transactions))
		for _, p := range in.Transactions {
			r, err := p.toRequest()
			if err != nil {
				return errResult(err), nil, nil
			}
			reqs = append(reqs, r)
		}
		out, err := h.svc.Transactions.Create(h.uid, reqs)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(out), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_transaction",
		Description: `Update transactions. Example args: {"transactions":[{"id":"<id>","name":"Updated"}]}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in updateTransactionsReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot update transactions"), nil, nil
		}
		reqs := make([]transactionservice.Request, 0, len(in.Transactions))
		for _, p := range in.Transactions {
			r, err := p.toRequest()
			if err != nil {
				return errResult(err), nil, nil
			}
			reqs = append(reqs, r)
		}
		out, err := h.svc.Transactions.Update(h.uid, reqs)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(out), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_transaction",
		Description: `Delete transactions by id. Example args: {"ids":["<id>"]}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in deleteTransactionsReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot delete transactions"), nil, nil
		}
		out, err := h.svc.Transactions.Delete(h.uid, in.IDs)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(out), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_account",
		Description: `Create an account. Example args: {"bankName":"Chase","endingNumbers":"1234","type":"CURRENT"}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createAccountReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot create accounts"), nil, nil
		}
		if !validAccountType(in.Type) {
			return errJSON("invalid account type"), nil, nil
		}
		acct, err := h.svc.Accounts.Create(h.uid, in.BankName, in.Nickname, in.Type, in.EndingNumbers)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(acct), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_account",
		Description: `Update an account by id. Example args: {"id":"<id>","bankName":"Chase Blue"}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in updateAccountReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot update accounts"), nil, nil
		}
		if !validAccountType(in.Type) {
			return errJSON("invalid account type"), nil, nil
		}
		acct, err := h.svc.Accounts.Update(h.uid, in.ID, in.BankName, in.Nickname, in.Type, in.EndingNumbers)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(acct), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_account",
		Description: `Delete an account by id. Example args: {"id":"<id>"}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in deleteReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot delete accounts"), nil, nil
		}
		if err := h.svc.Accounts.Delete(h.uid, in.ID); err != nil {
			return errResult(err), nil, nil
		}
		return okResult(map[string]bool{"ok": true}), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "link_transfers",
		Description: `Link transfer debit/credit pairs. Example args: {"links":[{"debitTransactionId":"<id>","creditTransactionId":"<id>"}]}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in transfersReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot link transfers"), nil, nil
		}
		links := make([][2]string, 0, len(in.Links))
		for _, l := range in.Links {
			links = append(links, [2]string{l.DebitTransactionID, l.CreditTransactionID})
		}
		out, err := h.svc.Transfers.LinkTransfers(h.uid, links)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(out), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "unlink_transfers",
		Description: `Unlink transfers by link id. Example args: {"ids":["<link-id>"]}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in transfersReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot unlink transfers"), nil, nil
		}
		out, err := h.svc.Transfers.UnlinkTransfers(h.uid, in.IDs)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(out), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_transfer_counterpart",
		Description: `Create the missing side of a transfer (counterpart). Example args: {"transactionId":"<id>","toAccountId":"<id>"}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createTransferCounterpartReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot create transfer counterpart"), nil, nil
		}
		out, err := h.svc.Transfers.CreateCounterpart(h.uid, in.TransactionID, in.ToAccountID)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(out), nil, nil
	})

	// --- mail ---
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_mailboxes",
		Description: "List linked mailboxes. Example: no arguments.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		rows, err := h.svc.Mail.List(h.uid)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(rows), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_mail_messages",
		Description: `List queued mail with the extracted fields. Example args: {"status":"PENDING","limit":20}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listMailReq) (*mcp.CallToolResult, any, error) {
		var status *string
		if strings.TrimSpace(in.Status) != "" {
			v := strings.ToUpper(strings.TrimSpace(in.Status))
			status = &v
		}
		limit := in.Limit
		if limit <= 0 {
			limit = 50
		}
		if limit > 200 {
			limit = 200
		}
		rows, total, _, err := h.svc.Mail.Messages(h.uid, status, in.AccountID, in.Search, "", limit)
		if err != nil {
			return errResult(err), nil, nil
		}
		out := make([]mailView, 0, len(rows))
		for _, row := range rows {
			out = append(out, mailViewOf(row, false))
		}
		return okResult(map[string]any{"messages": out, "total": total}), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_mail_message",
		Description: `Read one mail message, including the decoded body. Example args: {"id":"<message-id>"}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in mailIDReq) (*mcp.CallToolResult, any, error) {
		row, err := h.svc.Mail.Get(h.uid, in.ID)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(mailViewOf(row, true)), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "act_on_mail",
		Description: `Approve or reject mail, in bulk. Example args: {"messages":[{"id":"<id>","action":"REJECT"},{"id":"<id2>","action":"APPROVE","accountId":"<acct>"}]}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in mailActReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot act on mail"), nil, nil
		}
		if len(in.Messages) == 0 {
			return errJSON("no messages given"), nil, nil
		}
		if len(in.Messages) > 500 {
			return errJSON("at most 500 messages per call"), nil, nil
		}
		items := make([]model.MailActionItem, 0, len(in.Messages))
		for _, m := range in.Messages {
			action := strings.ToUpper(strings.TrimSpace(m.Action))
			if action != "APPROVE" && action != "REJECT" {
				return errJSON("action must be APPROVE or REJECT, got " + m.Action), nil, nil
			}
			items = append(items, model.MailActionItem{
				ID: m.ID, Action: action, AccountID: m.AccountID, CategoryIDs: m.CategoryIDs,
			})
		}
		out, err := h.svc.Mail.Act(h.uid, items)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(out), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "reject_all_mail",
		Description: `Reject every queued message matching a filter (max 200). Requires {"confirm":true}. Example args: {"confirm":true,"status":"PENDING"}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in rejectAllMailReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot reject mail"), nil, nil
		}
		if !in.Confirm {
			return errJSON(`refusing to reject mail without "confirm": true`), nil, nil
		}
		status := strings.ToUpper(strings.TrimSpace(in.Status))
		if status == "" {
			status = "PENDING"
		}
		rows, _, _, err := h.svc.Mail.Messages(h.uid, &status, in.AccountID, in.Search, "", 200)
		if err != nil {
			return errResult(err), nil, nil
		}
		items := make([]model.MailActionItem, 0, len(rows))
		for _, row := range rows {
			items = append(items, model.MailActionItem{ID: row.ID, Action: "REJECT"})
		}
		if len(items) == 0 {
			return okResult(map[string]any{"rejected": 0}), nil, nil
		}
		out, err := h.svc.Mail.Act(h.uid, items)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(map[string]any{"rejected": len(out.Succeeded), "failed": out.Failed}), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_mail_messages",
		Description: `Delete stored mail by id, or everything with {"all":true,"confirm":true}. Transactions created from mail are kept. Example args: {"ids":["<id>"]}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in deleteMailReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot delete mail"), nil, nil
		}
		if in.All {
			if !in.Confirm {
				return errJSON(`refusing to delete every mail without "confirm": true`), nil, nil
			}
			n, err := h.svc.Mail.DeleteAllMessages(h.uid)
			if err != nil {
				return errResult(err), nil, nil
			}
			return okResult(map[string]int64{"deleted": n}), nil, nil
		}
		if len(in.IDs) == 0 {
			return errJSON("give ids, or all+confirm to delete everything"), nil, nil
		}
		n, err := h.svc.Mail.DeleteMessages(h.uid, in.IDs)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(map[string]int64{"deleted": n}), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_mail_rules",
		Description: "List mail rules with their manual templates. Example: no arguments.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		rows, err := h.svc.Mail.Rules(h.uid)
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(rows), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_mail_rule",
		Description: `Create a mail rule. Example args: {"fromPattern":"hdfcbank","action":"REVIEW","template":{"amount":["Rs\\.?\\s*(?P<amount>[\\d,]+(?:\\.\\d{1,2})?)"],"debit":["(?i)\\bdebited\\b"],"date":["(?i)\\bon\\s+(?P<date>\\d{2}-\\d{2}-\\d{2})"]}}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in mailRuleReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot create mail rules"), nil, nil
		}
		rule, err := h.svc.Mail.CreateRule(h.uid, in.toReq())
		if err != nil {
			return errResult(err), nil, nil
		}
		return okResult(rule), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_mail_rule",
		Description: `Update a mail rule by id; a null template removes it. Example args: {"id":"<rule-id>","action":"AUTO"}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in updateMailRuleReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot update mail rules"), nil, nil
		}
		if err := h.svc.Mail.UpdateRule(h.uid, in.ID, in.toReq()); err != nil {
			return errResult(err), nil, nil
		}
		return okResult(map[string]bool{"ok": true}), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_mail_rule",
		Description: `Delete a mail rule by id. Example args: {"id":"<rule-id>"}.`,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in deleteReq) (*mcp.CallToolResult, any, error) {
		if !h.canWrite() {
			return errJSON("read-only API key; cannot delete mail rules"), nil, nil
		}
		if err := h.svc.Mail.DeleteRule(h.uid, in.ID); err != nil {
			return errResult(err), nil, nil
		}
		return okResult(map[string]bool{"ok": true}), nil, nil
	})

	return s
}

// mailView is the agent-facing shape of a message: parsed fields, not raw JSON.
type mailView struct {
	ID            string   `json:"id"`
	MessageID     string   `json:"messageId"`
	MailAccountID string   `json:"mailAccountId"`
	From          string   `json:"from"`
	FromName      string   `json:"fromName,omitempty"`
	Subject       string   `json:"subject"`
	ReceivedAt    *string  `json:"receivedAt"`
	Status        string   `json:"status"`
	Amount        *float64 `json:"amount,omitempty"`
	Direction     string   `json:"direction,omitempty"`
	OccurredAt    string   `json:"occurredAt,omitempty"`
	AccountHint   string   `json:"accountHint,omitempty"`
	Merchant      string   `json:"merchant,omitempty"`
	Confidence    float64  `json:"confidence"`
	TransactionID *string  `json:"transactionId,omitempty"`
	BodyPreview   string   `json:"bodyPreview,omitempty"`
	BodyText      *string  `json:"bodyText,omitempty"`
	RawText       *string  `json:"rawText,omitempty"`
}

func mailViewOf(row *model.MailMessage, withBody bool) mailView {
	v := mailView{
		ID: row.ID, MessageID: row.MessageID, MailAccountID: row.MailAccountID,
		From: row.FromAddr, FromName: row.FromName, Subject: row.Subject,
		ReceivedAt: row.ReceivedAt, Status: row.Status, Confidence: row.Confidence,
		TransactionID: row.TransactionID,
	}
	var parsed struct {
		Amount      *float64 `json:"amount"`
		Direction   *string  `json:"direction"`
		OccurredAt  *string  `json:"occurredAt"`
		AccountHint *string  `json:"accountHint"`
		Merchant    *string  `json:"merchant"`
	}
	if row.ParsedJSON != nil && json.Unmarshal([]byte(*row.ParsedJSON), &parsed) == nil {
		v.Amount = parsed.Amount
		if parsed.Direction != nil {
			v.Direction = *parsed.Direction
		}
		if parsed.OccurredAt != nil {
			v.OccurredAt = *parsed.OccurredAt
		}
		if parsed.AccountHint != nil {
			v.AccountHint = *parsed.AccountHint
		}
		if parsed.Merchant != nil {
			v.Merchant = *parsed.Merchant
		}
	}
	if withBody {
		v.BodyText = row.BodyText
		v.RawText = row.RawText
	} else {
		v.BodyPreview = row.BodyPreview
	}
	return v
}

func okResult(v any) *mcp.CallToolResult {
	b, err := json.Marshal(v)
	if err != nil {
		return errJSON("serialize error: " + err.Error())
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}
}

func errResult(err error) *mcp.CallToolResult {
	return errJSON(err.Error())
}

func errJSON(msg string) *mcp.CallToolResult {
	b, _ := json.Marshal(map[string]string{"error": msg})
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}
}

func logicOf(v string) string {
	if strings.EqualFold(v, string(model.RuleLogicAnd)) {
		return string(model.RuleLogicAnd)
	}
	return string(model.RuleLogicOr)
}

// normalizeOccurredAt accepts RFC3339 or YYYY-MM-DD and returns a stored
// timestamp; anything else is passed through unchanged.
func normalizeOccurredAt(v string) string {
	if v == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return tim.FromUTC(t.UTC())
	}
	if d, err := time.Parse("2006-01-02", v); err == nil {
		return tim.FromUTC(d.UTC())
	}
	return v
}

func categoryInputs(names []string) []repo.CategoryCreateInput {
	out := make([]repo.CategoryCreateInput, 0, len(names))
	for _, n := range names {
		if n != "" {
			out = append(out, repo.CategoryCreateInput{Name: n})
		}
	}
	return out
}
