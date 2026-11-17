package mail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fakeLiteLLM(t *testing.T, content string, status int) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing auth header: %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]string{"content": content},
			}},
		})
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func testSpecJSON(spec TemplateSpec) string {
	payload := map[string]any{"templates": []TemplateSpec{spec}}
	b, _ := json.Marshal(payload)
	return string(b)
}

func TestGenerateTemplatesValidatesSamples(t *testing.T) {
	spec := debitTemplate("alerts@bank.com")
	srv, _ := fakeLiteLLM(t, testSpecJSON(spec), http.StatusOK)
	samples := []LLMSample{{
		From: "alerts@bank.com", Subject: "Debited",
		Body: "Your A/c XX2467 has been debited INR 378.64 on 17 NOV 2026.",
	}}
	got, err := GenerateTemplates(context.Background(), LLMSettings{
		BaseURL: srv.URL, Model: "deepseek", APIKey: "test-key", Timeout: 5 * time.Second,
	}, samples)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Patterns.Amount == "" {
		t.Fatalf("templates = %+v", got)
	}
}

func TestGenerateTemplatesRejectsMismatchedTemplate(t *testing.T) {
	spec := debitTemplate("alerts@bank.com")
	srv, _ := fakeLiteLLM(t, testSpecJSON(spec), http.StatusOK)
	samples := []LLMSample{{
		From: "alerts@bank.com", Subject: "Debited",
		Body: "A completely different email with no numbers.",
	}}
	if _, err := GenerateTemplates(context.Background(), LLMSettings{
		BaseURL: srv.URL, Model: "deepseek", APIKey: "test-key", Timeout: 5 * time.Second,
	}, samples); err == nil {
		t.Fatal("template that does not parse the sample must be rejected")
	}
}

func TestGenerateTemplatesHandlesHTTPError(t *testing.T) {
	srv, _ := fakeLiteLLM(t, "nope", http.StatusInternalServerError)
	if _, err := GenerateTemplates(context.Background(), LLMSettings{
		BaseURL: srv.URL, Model: "deepseek", APIKey: "test-key", Timeout: 5 * time.Second,
	}, []LLMSample{{Subject: "x", Body: "INR 1 debited"}}); err == nil {
		t.Fatal("HTTP error must surface")
	}
}

func TestGenerateTemplatesRequiresConfig(t *testing.T) {
	if _, err := GenerateTemplates(context.Background(), LLMSettings{}, []LLMSample{{}}); err == nil {
		t.Fatal("missing base URL must fail")
	}
	if _, err := GenerateTemplates(context.Background(), LLMSettings{BaseURL: "http://x"}, []LLMSample{{}}); err == nil {
		t.Fatal("missing model must fail")
	}
}

func TestStripCodeFence(t *testing.T) {
	in := "```json\n{\"templates\":[]}\n```"
	if got := stripCodeFence(in); got != `{"templates":[]}` {
		t.Fatalf("stripCodeFence = %q", got)
	}
}

func TestChatEndpoint(t *testing.T) {
	cases := map[string]string{
		"https://api.deepseek.com":                     "https://api.deepseek.com/chat/completions",
		"https://api.deepseek.com/v1/":                 "https://api.deepseek.com/v1/chat/completions",
		"https://api.deepseek.com/v1/chat/completions": "https://api.deepseek.com/v1/chat/completions",
		"http://litellm:4000/v1":                       "http://litellm:4000/v1/chat/completions",
	}
	for in, want := range cases {
		if got := chatEndpoint(in); got != want {
			t.Errorf("chatEndpoint(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestChatPayloadIncludesReasoning(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": `{"templates":[]}`}}},
		})
	}))
	defer srv.Close()
	_, _ = GenerateTemplates(context.Background(), LLMSettings{
		BaseURL: srv.URL, Model: "deepseek-flash", APIKey: "k",
		ReasoningEnabled: true, ReasoningEffort: "high", Timeout: 5 * time.Second,
	}, []LLMSample{{From: "a@b.c", Subject: "s", Body: "b"}})
	if _, ok := got["thinking"]; !ok {
		t.Fatal("thinking was not sent")
	}
	if got["reasoning_effort"] != "high" {
		t.Fatalf("reasoning_effort = %v", got["reasoning_effort"])
	}
}

func TestGenerateTemplatesReportsValidationReason(t *testing.T) {
	spec := debitTemplate("alerts@bank.com")
	srv, _ := fakeLiteLLM(t, testSpecJSON(spec), http.StatusOK)
	_, err := GenerateTemplates(context.Background(), LLMSettings{
		BaseURL: srv.URL, Model: "deepseek", APIKey: "test-key", Timeout: 5 * time.Second,
	}, []LLMSample{{From: "alerts@bank.com", Subject: "Debited", Body: "No numbers here."}})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "sample 1") {
		t.Fatalf("error should name the failing sample: %v", err)
	}
}
