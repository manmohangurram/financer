package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mohan9182/financer/internal/netguard"
)

// LLMSettings is the transport configuration for one user's OpenAI-compatible
// endpoint (LiteLLM, DeepSeek direct, OpenAI, Ollama).
type LLMSettings struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration

	// Reasoning controls are sent only when enabled (DeepSeek thinking mode).
	ReasoningEnabled bool
	ReasoningEffort  string
}

// LLMSample is one email given to the model as evidence.
type LLMSample struct {
	From    string
	Subject string
	Body    string
}

const templateSystemPrompt = `You generate parsing templates for bank transaction alert emails.
Return ONLY JSON: {"templates":[{"name":string,"guards":{"subject_contains":string,"body_contains":string,"body_not_contains":[string]},"patterns":{"amount":string,"debit":string,"credit":string,"account":string,"merchant":string,"date":string}}]}.
Patterns are Go RE2 regexes. amount/date are required; debit or credit is required; account/merchant are optional.
Use named capture groups amount, account, merchant, date where possible. Guards must be short literal substrings, not regexes.
Every template must parse every sample into amount (>0), direction, and date. Do not invent fields.`

// GenerateTemplates asks the model for template variants and validates each one
// against every sample before returning it.
func GenerateTemplates(ctx context.Context, settings LLMSettings, samples []LLMSample) ([]TemplateSpec, error) {
	if strings.TrimSpace(settings.BaseURL) == "" {
		return nil, fmt.Errorf("LLM base URL is not configured")
	}
	if strings.TrimSpace(settings.Model) == "" {
		return nil, fmt.Errorf("LLM model is not configured")
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("no samples provided")
	}
	if len(samples) > 3 {
		samples = samples[:3]
	}

	var sb strings.Builder
	sb.WriteString("\n\nSamples:\n")
	for i, s := range samples {
		body := s.Body
		if len(body) > 4000 {
			body = body[:4000]
		}
		fmt.Fprintf(&sb, "\n--- SAMPLE %d ---\nFrom: %s\nSubject: %s\n\n%s\n", i+1, s.From, s.Subject, body)
	}

	content, err := chat(ctx, settings, []map[string]string{
		{"role": "system", "content": templateSystemPrompt},
		{"role": "user", "content": sb.String()},
	})
	if err != nil {
		return nil, err
	}
	content = stripCodeFence(content)

	var payload struct {
		Templates []TemplateSpec `json:"templates"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		// Some models return a bare array.
		var direct []TemplateSpec
		if err2 := json.Unmarshal([]byte(content), &direct); err2 != nil {
			return nil, fmt.Errorf("LLM returned unreadable JSON: %w", err)
		}
		payload.Templates = direct
	}

	var accepted []TemplateSpec
	seen := map[string]bool{}
	for _, spec := range payload.Templates {
		tpl, err := CompileTemplate(spec)
		if err != nil {
			continue
		}
		ok := true
		for _, sample := range samples {
			match, matched := tpl.Extract(sample.Subject, sample.Body)
			if !matched || match == nil || !match.Complete || match.Extraction == nil {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		key := spec.Name + "|" + spec.Guards.SubjectContains + "|" + spec.Guards.BodyContains + "|" + spec.Patterns.Amount
		if seen[key] {
			continue
		}
		seen[key] = true
		accepted = append(accepted, spec)
	}
	if len(accepted) == 0 {
		return nil, fmt.Errorf("LLM produced no template that parses every sample")
	}
	return accepted, nil
}

// TestLLM performs a tiny request to verify the endpoint/model/key.
func TestLLM(ctx context.Context, settings LLMSettings) error {
	content, err := chat(ctx, settings, []map[string]string{
		{"role": "user", "content": `Reply with this json only: {"ok":true}`},
	})
	if err != nil {
		return err
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("empty response from the model")
	}
	return nil
}

func chat(ctx context.Context, settings LLMSettings, messages []map[string]string) (string, error) {
	timeout := settings.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	// Thinking models are slower than a plain chat: never use less than 2 min.
	if settings.ReasoningEnabled && timeout < 120*time.Second {
		timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload := map[string]any{
		"model":       settings.Model,
		"messages":    messages,
		"temperature": 0,
		"response_format": map[string]string{
			"type": "json_object",
		},
	}
	if settings.ReasoningEnabled {
		payload["thinking"] = map[string]string{"type": "enabled"}
		if settings.ReasoningEffort != "" {
			payload["reasoning_effort"] = settings.ReasoningEffort
		}
	}
	endpoint := chatEndpoint(settings.BaseURL)
	target, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("building LLM request: %w", err)
	}
	if err := netguard.Resolve(target.Hostname()); err != nil {
		return "", err
	}
	attempt := func(withJSONMode bool) (*http.Response, []byte, error) {
		payloadCopy := make(map[string]any, len(payload))
		for k, v := range payload {
			payloadCopy[k] = v
		}
		if !withJSONMode {
			delete(payloadCopy, "response_format")
		}
		body, marshalErr := json.Marshal(payloadCopy)
		if marshalErr != nil {
			return nil, nil, marshalErr
		}
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if reqErr != nil {
			return nil, nil, fmt.Errorf("building LLM request: %w", reqErr)
		}
		req.Header.Set("Content-Type", "application/json")
		if settings.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+settings.APIKey)
		}
		resp, doErr := http.DefaultClient.Do(req)
		if doErr != nil {
			return nil, nil, fmt.Errorf("LLM request failed: %w", doErr)
		}
		defer resp.Body.Close()
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if readErr != nil {
			return resp, nil, fmt.Errorf("reading LLM response: %w", readErr)
		}
		return resp, raw, nil
	}

	resp, raw, err := attempt(true)
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusBadRequest && bytes.Contains(raw, []byte("response_format")) {
		// Some reasoning endpoints reject JSON mode; retry once without it.
		resp, raw, err = attempt(false)
		if err != nil {
			return "", err
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("LLM returned HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}

	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", fmt.Errorf("decoding LLM response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("LLM response had no choices")
	}
	return decoded.Choices[0].Message.Content, nil
}

// chatEndpoint accepts either a base URL (…/v1) or a full completions URL, so a
// pasted endpoint from a provider's docs works without editing.
func chatEndpoint(base string) string {
	b := strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(b, "/chat/completions") {
		return b
	}
	return b + "/chat/completions"
}

func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
