package llmsettings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/mohan9182/financer/internal/db"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/repo/sqlite"
)

const testSecret = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func setup(t *testing.T) (*Service, string, func()) {
	t.Helper()
	dbs, err := db.Open(filepath.Join(t.TempDir(), "financer.db"))
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	users := sqlite.NewUserRepo(dbs.Write)
	uid, err := users.Create("ai@test.com", "hash", "AI")
	if err != nil {
		t.Fatal(err)
	}
	secret := testSecret
	return New(sqlite.NewLLMSettingsRepo(dbs.Write), &secret), uid, func() {
		dbs.Write.Close()
		dbs.Read.Close()
	}
}

func keyJSON(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

func TestDefaultsAndKeyLifecycle(t *testing.T) {
	svc, uid, cleanup := setup(t)
	defer cleanup()

	got, err := svc.Get(uid)
	if err != nil {
		t.Fatal(err)
	}
	if got.Enabled != true || got.TimeoutSecs != 20 || got.MinSamples != 3 || got.APIKeySet {
		t.Fatalf("defaults = %+v", got)
	}

	enabled := true
	updated, err := svc.Update(uid, model.ReqLLMSettings{
		BaseURL: "http://litellm:4000/v1", Model: "deepseek-chat",
		APIKey: keyJSON("sk-secret"), TimeoutSecs: 30, Enabled: &enabled,
		MinSamples: 4, FailureThreshold: 5, CooldownMinutes: 90, AutoPollThrottleMinutes: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.APIKeySet || updated.BaseURL != "http://litellm:4000/v1" {
		t.Fatalf("updated = %+v", updated)
	}
	// The plaintext key is never in the stored row or the wire value.
	row, err := svc.Stored(uid)
	if err != nil {
		t.Fatal(err)
	}
	if row.APIKeyEnc == "" || row.APIKeyEnc == "sk-secret" {
		t.Fatalf("api_key_enc = %q", row.APIKeyEnc)
	}
	cfg, err := svc.Config(uid)
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || cfg.APIKey != "sk-secret" || cfg.Model != "deepseek-chat" {
		t.Fatalf("config = %+v", cfg)
	}

	// Omitted key keeps it; null clears it.
	updated, err = svc.Update(uid, model.ReqLLMSettings{
		BaseURL: "http://litellm:4000/v1", Model: "deepseek-chat", Enabled: &enabled,
	})
	if err != nil || !updated.APIKeySet {
		t.Fatalf("keep key: %+v err=%v", updated, err)
	}
	updated, err = svc.Update(uid, model.ReqLLMSettings{
		BaseURL: "http://litellm:4000/v1", Model: "deepseek-chat",
		APIKey: json.RawMessage("null"), Enabled: &enabled,
	})
	if err != nil || updated.APIKeySet {
		t.Fatalf("clear key: %+v err=%v", updated, err)
	}
}

func TestUpdateValidation(t *testing.T) {
	svc, uid, cleanup := setup(t)
	defer cleanup()
	enabled := true
	cases := []model.ReqLLMSettings{
		{BaseURL: "", Model: "m", Enabled: &enabled},
		{BaseURL: "ftp://x", Model: "m", Enabled: &enabled},
		{BaseURL: "http://x", Model: "", Enabled: &enabled},
		{BaseURL: "http://user:pass@x", Model: "m", Enabled: &enabled},
	}
	for i, req := range cases {
		if _, err := svc.Update(uid, req); err == nil {
			t.Errorf("case %d should fail", i)
		}
	}
	// Thresholds are clamped to their ranges.
	updated, err := svc.Update(uid, model.ReqLLMSettings{
		BaseURL: "http://x", Model: "m", Enabled: &enabled,
		TimeoutSecs: 1, MinSamples: 99, FailureThreshold: 0, CooldownMinutes: 1, AutoPollThrottleMinutes: 999,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.TimeoutSecs != 5 || updated.MinSamples != 10 || updated.FailureThreshold != 3 || updated.CooldownMinutes != 5 || updated.AutoPollThrottleMinutes != 60 {
		t.Fatalf("clamped = %+v", updated)
	}
}

func TestKeyRequiresSecret(t *testing.T) {
	dbs, err := db.Open(filepath.Join(t.TempDir(), "financer.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dbs.Write.Close()
	defer dbs.Read.Close()
	users := sqlite.NewUserRepo(dbs.Write)
	uid, _ := users.Create("no@test.com", "h", "No")
	svc := New(sqlite.NewLLMSettingsRepo(dbs.Write), nil)
	enabled := true
	if _, err := svc.Update(uid, model.ReqLLMSettings{
		BaseURL: "http://x", Model: "m", APIKey: keyJSON("secret"), Enabled: &enabled,
	}); err == nil {
		t.Fatal("storing a key without FINANCER_SECRET_KEY must fail")
	}
}

func TestConnectionTest(t *testing.T) {
	svc, uid, cleanup := setup(t)
	defer cleanup()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": `{"ok":true}`}}},
		})
	}))
	defer srv.Close()
	enabled := true
	if _, err := svc.Update(uid, model.ReqLLMSettings{
		BaseURL: srv.URL, Model: "deepseek-chat", Enabled: &enabled,
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Test(context.Background(), uid); err != nil {
		t.Fatalf("test failed: %v", err)
	}
}
