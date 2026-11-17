package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/mohan9182/financer/internal/accountservice"
	"github.com/mohan9182/financer/internal/auth"
	"github.com/mohan9182/financer/internal/authservice"
	"github.com/mohan9182/financer/internal/categoryservice"
	"github.com/mohan9182/financer/internal/db"
	"github.com/mohan9182/financer/internal/keyservice"
	"github.com/mohan9182/financer/internal/mailservice"
	"github.com/mohan9182/financer/internal/repo/sqlite"
	"github.com/mohan9182/financer/internal/ruleservice"
	"github.com/mohan9182/financer/internal/transactionservice"
	"github.com/mohan9182/financer/internal/transferservice"
	"github.com/mohan9182/financer/internal/userservice"
)

type testServer struct {
	api    *API
	keySvc *keyservice.Service
	uid    string
	url    string
	close  func()
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	dbs, err := db.Open(filepath.Join(t.TempDir(), "financer.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	users := sqlite.NewUserRepo(dbs.Write)
	accounts := sqlite.NewAccountRepo(dbs.Write)
	categories := sqlite.NewCategoryRepo(dbs.Write)
	txns := sqlite.NewTransactionRepo(dbs.Write)
	rules := sqlite.NewRuleRepo(dbs.Write)
	mails := sqlite.NewMailRepo(dbs.Write)
	keys := sqlite.NewUserKeyRepo(dbs.Write)

	uid, err := users.Create("mcp@test.com", "hash", "MCP")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accounts.Create(uid, "HDFC", "Main", "SAVINGS", "1234"); err != nil {
		t.Fatal(err)
	}

	jwt := auth.New("secret")
	authSvc := authservice.New(users, jwt)
	userSvc := userservice.New(users, jwt, t.TempDir())
	accountSvc := accountservice.New(accounts)
	categorySvc := categoryservice.New(categories)
	transferSvc := transferservice.New(txns, accounts)
	ruleSvc := ruleservice.New(rules, categories, txns)
	transferRuleSvc := ruleservice.NewTransferRuleService(rules, txns, accounts, transferSvc)
	txnSvc := transactionservice.New(txns, accounts, categories).WithRule(ruleSvc).WithTransferRule(transferRuleSvc)
	keySvc := keyservice.New(keys)
	mailSvc := mailservice.New(mails, nil, txnSvc, accountSvc)

	api := New(Deps{
		Auth: authSvc, User: userSvc, Account: accountSvc, Category: categorySvc,
		Transaction: txnSvc, Transfer: transferSvc, Rule: ruleSvc,
		TransferRule: transferRuleSvc, Mail: mailSvc, Keys: keySvc,
		JWTSecret: "secret", StaticDir: t.TempDir(), AvatarDir: t.TempDir(),
		AllowInsecure: true,
	})
	srv := httptest.NewServer(api.Handler())
	return &testServer{
		api: api, keySvc: keySvc, uid: uid, url: srv.URL,
		close: func() {
			srv.Close()
			dbs.Write.Close()
			dbs.Read.Close()
		},
	}
}

type bearerTransport struct {
	token string
}

func (b bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(clone)
}

func connectMCP(t *testing.T, url, key string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1.0.0"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:             url + "/mcp",
		HTTPClient:           &http.Client{Transport: bearerTransport{token: key}},
		DisableStandaloneSSE: true,
	}
	session, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatalf("mcp connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func textOf(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatal("tool returned no content")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type %T", res.Content[0])
	}
	return tc.Text
}

func TestMCPReadToolsAndScope(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	readKey, err := ts.keySvc.Create(ts.uid, "read", 0, "read")
	if err != nil {
		t.Fatal(err)
	}
	session := connectMCP(t, ts.url, readKey.Key)

	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) < 20 {
		t.Fatalf("tools = %d", len(tools.Tools))
	}

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_accounts"})
	if err != nil {
		t.Fatal(err)
	}
	text := textOf(t, res)
	if !strings.Contains(text, "HDFC") {
		t.Fatalf("list_accounts = %s", text)
	}

	res, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_balance_summary"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textOf(t, res), "totalBalance") {
		t.Fatalf("balance = %s", textOf(t, res))
	}

	// A read key cannot write.
	res, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "create_transaction",
		Arguments: map[string]any{"transactions": []any{map[string]any{
			"name": "Coffee", "amount": 4.5, "type": "DEBIT", "accountId": "x",
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textOf(t, res), "read-only API key") {
		t.Fatalf("create_transaction = %s", textOf(t, res))
	}
}

func TestMCPWriteKeyCreatesTransaction(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	writeKey, err := ts.keySvc.Create(ts.uid, "rw", 0, "read_write")
	if err != nil {
		t.Fatal(err)
	}
	session := connectMCP(t, ts.url, writeKey.Key)

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "list_accounts",
	})
	if err != nil {
		t.Fatal(err)
	}
	var accounts []map[string]any
	if err := json.Unmarshal([]byte(textOf(t, res)), &accounts); err != nil {
		t.Fatal(err)
	}
	accountID := accounts[0]["id"].(string)

	res, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "create_transaction",
		Arguments: map[string]any{"transactions": []any{map[string]any{
			"name": "Coffee", "amount": 4.5, "type": "DEBIT", "accountId": accountID,
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textOf(t, res), `"success":true`) {
		t.Fatalf("create_transaction = %s", textOf(t, res))
	}

	res, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_transactions"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(textOf(t, res), "Coffee") {
		t.Fatalf("list_transactions = %s", textOf(t, res))
	}
}

func TestMCPRejectsMissingOrBadKey(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	for _, key := range []string{"", "fin_live_wrong"} {
		req, _ := http.NewRequest(http.MethodPost, ts.url+"/mcp", strings.NewReader("{}"))
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("key %q -> %d %s", key, resp.StatusCode, body)
		}
		if !strings.Contains(string(body), "invalid or missing API key") {
			t.Fatalf("body = %s", body)
		}
	}
}

func TestOpenAPISpecAndDocs(t *testing.T) {
	ts := newTestServer(t)
	defer ts.close()

	resp, err := http.Get(ts.url + "/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("spec status = %d", resp.StatusCode)
	}
	var spec map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		t.Fatal(err)
	}
	if spec["openapi"] == "" {
		t.Fatal("missing openapi version")
	}
	paths := spec["paths"].(map[string]any)
	for _, want := range []string{
		"/api/auth/signup", "/api/transactions", "/api/rules", "/api/dashboard",
		"/api/spending", "/api/me/keys", "/api/mail/accounts",
	} {
		if _, ok := paths[want]; !ok {
			t.Errorf("spec missing path %s", want)
		}
	}
	if _, ok := paths["/api/transactions/import/file"]; ok {
		t.Error("removed import endpoint must not be documented")
	}

	resp2, err := http.Get(ts.url + "/docs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	body, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(body), "SwaggerUIBundle") {
		t.Fatal("docs page does not load Swagger UI")
	}
}
