package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/mohan9182/financer/internal/mail"
	"github.com/mohan9182/financer/internal/repo"
)

func seedMCPMail(t *testing.T, ts *testServer, messageID, from string, amount float64) string {
	t.Helper()
	acct, err := ts.mailRepo.CreateAccount(ts.uid, &repo.NewMailAccount{
		Label: "P", IMAPHost: "imap.x.com", IMAPPort: 993,
		Username: "a@x.com", SecretEnc: "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	occurred := "2026-11-12"
	hint := "1234"
	extraction := mail.Extraction{
		Amount: amount, Direction: mail.DirectionDebit, OccurredAt: &occurred,
		Merchant: "BALAJI WINES", AccountHint: &hint, Confidence: 1,
	}
	parsed, _ := json.Marshal(extraction)
	parsedStr := string(parsed)
	raw := "Rs.250.00 is debited from your account ending 1234 on 12-11-26."
	if _, err := ts.mailRepo.InsertMessage(ts.uid, &repo.NewMailMessage{
		MailAccountID: acct.ID, MessageID: messageID, FromAddr: from, FromName: "Bank",
		Subject: "Debit Alert", RawText: raw, BodyText: raw,
		Status: repo.MailStatusPending, ParsedJSON: &parsedStr, Confidence: 1,
		ExtractorSource: "template",
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := ts.mailRepo.ListMessages(ts.uid, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.MessageID == messageID {
			return row.ID
		}
	}
	t.Fatalf("seeded mail %s not found", messageID)
	return ""
}

func TestMCPMailReadTools(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()
	seedMCPMail(t, ts, "<mail1>", "alerts@hdfcbank.net", 250)

	readKey, err := ts.keySvc.Create(ts.uid, "read", 0, "read")
	if err != nil {
		t.Fatal(err)
	}
	session := connectMCP(t, ts.url, readKey.Key)
	ctx := context.Background()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_mailboxes"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textOf(t, res), "imap.x.com") {
		t.Fatalf("list_mailboxes = %s", textOf(t, res))
	}

	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_mail_messages",
		Arguments: map[string]any{"status": "PENDING"},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := textOf(t, res)
	if !strings.Contains(text, "alerts@hdfcbank.net") || !strings.Contains(text, `"amount":250`) {
		t.Fatalf("list_mail_messages = %s", text)
	}
	var list struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal([]byte(text), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Messages) != 1 || list.Total != 1 {
		t.Fatalf("list = %+v", list)
	}

	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_mail_message",
		Arguments: map[string]any{"id": list.Messages[0].ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textOf(t, res), "bodyText") {
		t.Fatalf("get_mail_message = %s", textOf(t, res))
	}

	// A read key cannot act on mail.
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "act_on_mail",
		Arguments: map[string]any{"messages": []any{
			map[string]any{"id": list.Messages[0].ID, "action": "REJECT"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textOf(t, res), "read-only API key") {
		t.Fatalf("act_on_mail = %s", textOf(t, res))
	}
}

func TestMCPMailWriteTools(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()
	id := seedMCPMail(t, ts, "<mail2>", "alerts@hdfcbank.net", 250)
	seedMCPMail(t, ts, "<mail3>", "alerts@hdfcbank.net", 99)

	writeKey, err := ts.keySvc.Create(ts.uid, "rw", 0, "read_write")
	if err != nil {
		t.Fatal(err)
	}
	session := connectMCP(t, ts.url, writeKey.Key)
	ctx := context.Background()

	// Create a rule with a manual template.
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "create_mail_rule",
		Arguments: map[string]any{
			"fromPattern": "alerts@hdfcbank.net",
			"action":      "REVIEW",
			"template": map[string]any{
				"amount": []any{`Rs\.?\s*(?P<amount>[\d,]+(?:\.\d{1,2})?)`},
				"debit":  []any{`(?i)\bdebited\b`},
				"date":   []any{`(?i)\bon\s+(?P<date>\d{2}-\d{2}-\d{2})`},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := textOf(t, res)
	if !strings.Contains(text, "alerts@hdfcbank.net") || !strings.Contains(text, "template") {
		t.Fatalf("create_mail_rule = %s", text)
	}
	var rule struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(text), &rule); err != nil {
		t.Fatal(err)
	}

	res, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "list_mail_rules"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textOf(t, res), `"fromPattern":"alerts@hdfcbank.net"`) {
		t.Fatalf("list_mail_rules = %s", textOf(t, res))
	}

	// Bulk reject one message by id.
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "act_on_mail",
		Arguments: map[string]any{"messages": []any{
			map[string]any{"id": id, "action": "REJECT"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textOf(t, res), `"succeeded"`) {
		t.Fatalf("act_on_mail = %s", textOf(t, res))
	}

	// Reject-all refuses without confirm, then rejects the remaining PENDING mail.
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "reject_all_mail",
		Arguments: map[string]any{"status": "PENDING"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textOf(t, res), "confirm") {
		t.Fatalf("reject_all_mail without confirm = %s", textOf(t, res))
	}
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "reject_all_mail",
		Arguments: map[string]any{"confirm": true, "status": "PENDING"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textOf(t, res), `"rejected":1`) {
		t.Fatalf("reject_all_mail = %s", textOf(t, res))
	}

	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "delete_mail_rule",
		Arguments: map[string]any{"id": rule.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(textOf(t, res), `"error"`) {
		t.Fatalf("delete_mail_rule = %s", textOf(t, res))
	}
}
