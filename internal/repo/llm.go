package repo

// LLMSettingsRow is the stored per-user LiteLLM configuration.
type LLMSettingsRow struct {
	UserID                  string
	BaseURL                 string
	Model                   string
	APIKeyEnc               string
	TimeoutSecs             int64
	Enabled                 bool
	MinSamples              int64
	FailureThreshold        int64
	CooldownMinutes         int64
	AutoPollThrottleMinutes int64
	UpdatedAt               string
}

// LLMSettingsRepo stores one settings row per user.
type LLMSettingsRepo interface {
	Get(userID string) (*LLMSettingsRow, error)
	Upsert(row *LLMSettingsRow) error
}
