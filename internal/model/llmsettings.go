package model

import "encoding/json"

// LLMSettings is the per-user LiteLLM configuration returned to Settings. The
// API key is never included; apiKeySet only reports whether one is stored.
type LLMSettings struct {
	BaseURL                 string `json:"baseUrl"`
	Model                   string `json:"model"`
	TimeoutSecs             int64  `json:"timeoutSecs"`
	Enabled                 bool   `json:"enabled"`
	APIKeySet               bool   `json:"apiKeySet"`
	MinSamples              int64  `json:"minSamples"`
	FailureThreshold        int64  `json:"failureThreshold"`
	CooldownMinutes         int64  `json:"cooldownMinutes"`
	AutoPollThrottleMinutes int64  `json:"autoPollThrottleMinutes"`
	ReasoningEnabled        bool   `json:"reasoningEnabled"`
	ReasoningEffort         string `json:"reasoningEffort"`
	UpdatedAt               string `json:"updatedAt"`
}

// ReqLLMSettings updates the per-user settings. APIKey has three states:
// omitted/"" keeps the stored key, null clears it, any other string replaces it.
type ReqLLMSettings struct {
	BaseURL                 string          `json:"baseUrl"`
	Model                   string          `json:"model"`
	APIKey                  json.RawMessage `json:"apiKey"`
	TimeoutSecs             int64           `json:"timeoutSecs"`
	Enabled                 *bool           `json:"enabled"`
	MinSamples              int64           `json:"minSamples"`
	FailureThreshold        int64           `json:"failureThreshold"`
	CooldownMinutes         int64           `json:"cooldownMinutes"`
	AutoPollThrottleMinutes int64           `json:"autoPollThrottleMinutes"`
	ReasoningEnabled        *bool           `json:"reasoningEnabled"`
	ReasoningEffort         string          `json:"reasoningEffort"`
}

// LLMTestResult is the response of the settings test action.
type LLMTestResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}
