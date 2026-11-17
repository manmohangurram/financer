// Package llmsettings stores each user's LiteLLM endpoint/model/key and the
// template-learning thresholds. The key is encrypted with the same scheme as
// mailbox passwords and is never returned to the client.
package llmsettings

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/mail"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/netguard"
	"github.com/mohan9182/financer/internal/repo"
	tim "github.com/mohan9182/financer/internal/time"
)

const (
	defaultTimeout   = 20
	defaultSamples   = 3
	defaultFailures  = 3
	defaultCooldown  = 60
	defaultThrottle  = 5
	maxBaseURLLength = 300
)

type Service struct {
	repo   repo.LLMSettingsRepo
	secret *string
}

func New(r repo.LLMSettingsRepo, secret *string) *Service {
	return &Service{repo: r, secret: secret}
}

// Get returns the settings, or defaults with no endpoint/model configured.
func (s *Service) Get(userID string) (*model.LLMSettings, error) {
	row, err := s.repo.Get(userID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return &model.LLMSettings{
			TimeoutSecs: defaultTimeout, Enabled: true,
			MinSamples: defaultSamples, FailureThreshold: defaultFailures,
			CooldownMinutes: defaultCooldown, AutoPollThrottleMinutes: defaultThrottle,
		}, nil
	}
	return toWire(row), nil
}

// Update validates and stores the settings. An omitted/empty apiKey keeps the
// stored key; JSON null clears it.
func (s *Service) Update(userID string, req model.ReqLLMSettings) (*model.LLMSettings, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	modelName := strings.TrimSpace(req.Model)
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if enabled {
		if err := validateBaseURL(baseURL); err != nil {
			return nil, err
		}
		if modelName == "" {
			return nil, errors.BadRequest("model is required")
		}
	}
	if len(modelName) > 100 {
		return nil, errors.BadRequest("model must be at most 100 characters")
	}

	timeout := clampDefault(req.TimeoutSecs, defaultTimeout, 5, 300)
	samples := clampDefault(req.MinSamples, defaultSamples, 1, 10)
	failures := clampDefault(req.FailureThreshold, defaultFailures, 1, 10)
	cooldown := clampDefault(req.CooldownMinutes, defaultCooldown, 5, 1440)
	throttle := clampDefault(req.AutoPollThrottleMinutes, defaultThrottle, 1, 60)
	reasoning := false
	if req.ReasoningEnabled != nil {
		reasoning = *req.ReasoningEnabled
	}
	effort := strings.ToLower(strings.TrimSpace(req.ReasoningEffort))
	if effort != "" && effort != "low" && effort != "medium" && effort != "high" {
		return nil, errors.BadRequest("reasoningEffort must be low, medium or high")
	}
	if !reasoning {
		effort = ""
	}

	existing, err := s.repo.Get(userID)
	if err != nil {
		return nil, err
	}
	keyEnc := ""
	if existing != nil {
		keyEnc = existing.APIKeyEnc
	}
	state, keyValue, err := decodeAPIKey(req.APIKey)
	if err != nil {
		return nil, err
	}
	switch state {
	case keySet:
		if s.secret == nil {
			return nil, errors.Internal("FINANCER_SECRET_KEY is not set; cannot store the LLM API key")
		}
		enc, err := mail.Encrypt(*s.secret, keyValue)
		if err != nil {
			return nil, err
		}
		keyEnc = enc
	case keyClear:
		keyEnc = ""
	}

	row := &repo.LLMSettingsRow{
		UserID: userID, BaseURL: baseURL, Model: modelName, APIKeyEnc: keyEnc,
		TimeoutSecs: timeout, Enabled: enabled, MinSamples: samples,
		FailureThreshold: failures, CooldownMinutes: cooldown,
		AutoPollThrottleMinutes: throttle,
		ReasoningEnabled:        reasoning, ReasoningEffort: effort,
	}
	if err := s.repo.Upsert(row); err != nil {
		return nil, err
	}
	return toWire(row), nil
}

// Stored returns the raw settings row (nil when never configured).
func (s *Service) Stored(userID string) (*repo.LLMSettingsRow, error) {
	return s.repo.Get(userID)
}

// Config returns the executable transport config, or nil when generation is
// disabled/unconfigured.
func (s *Service) Config(userID string) (*mail.LLMSettings, error) {
	row, err := s.repo.Get(userID)
	if err != nil {
		return nil, err
	}
	if row == nil || !row.Enabled || row.BaseURL == "" || row.Model == "" {
		return nil, nil
	}
	key := ""
	if row.APIKeyEnc != "" {
		if s.secret == nil {
			return nil, errors.Internal("FINANCER_SECRET_KEY is not set; cannot read the LLM API key")
		}
		key, err = mail.Decrypt(*s.secret, row.APIKeyEnc)
		if err != nil {
			return nil, err
		}
	}
	return &mail.LLMSettings{
		BaseURL: row.BaseURL, APIKey: key, Model: row.Model,
		Timeout:          time.Duration(row.TimeoutSecs) * time.Second,
		ReasoningEnabled: row.ReasoningEnabled, ReasoningEffort: row.ReasoningEffort,
	}, nil
}

// Test calls the configured model with a tiny prompt.
func (s *Service) Test(ctx context.Context, userID string) error {
	cfg, err := s.Config(userID)
	if err != nil {
		return err
	}
	if cfg == nil {
		return errors.BadRequest("Inbox AI is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	return mail.TestLLM(ctx, *cfg)
}

func validateBaseURL(raw string) error {
	if raw == "" {
		return errors.BadRequest("base URL is required")
	}
	if len(raw) > maxBaseURLLength {
		return errors.BadRequest("base URL must be at most 300 characters")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.BadRequest("base URL must be an http(s) URL")
	}
	if u.User != nil {
		return errors.BadRequest("base URL must not contain credentials")
	}
	if netguard.IsBlockedHost(u.Hostname()) {
		return errors.BadRequest("base URL must not target a link-local or metadata address")
	}
	return nil
}

func clampDefault(v, def, min, max int64) int64 {
	if v == 0 {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

type keyState int

const (
	keyKeep keyState = iota
	keySet
	keyClear
)

func decodeAPIKey(raw json.RawMessage) (keyState, string, error) {
	if len(raw) == 0 {
		return keyKeep, "", nil
	}
	if strings.TrimSpace(string(raw)) == "null" {
		return keyClear, "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return keyKeep, "", errors.BadRequest("apiKey must be a string or null")
	}
	if strings.TrimSpace(value) == "" {
		return keyKeep, "", nil
	}
	return keySet, value, nil
}

func toWire(row *repo.LLMSettingsRow) *model.LLMSettings {
	return &model.LLMSettings{
		BaseURL: row.BaseURL, Model: row.Model, TimeoutSecs: row.TimeoutSecs,
		Enabled: row.Enabled, APIKeySet: row.APIKeyEnc != "",
		MinSamples: row.MinSamples, FailureThreshold: row.FailureThreshold,
		CooldownMinutes:         row.CooldownMinutes,
		AutoPollThrottleMinutes: row.AutoPollThrottleMinutes,
		ReasoningEnabled:        row.ReasoningEnabled,
		ReasoningEffort:         row.ReasoningEffort,
		UpdatedAt:               tim.TSRFC3339(row.UpdatedAt),
	}
}
