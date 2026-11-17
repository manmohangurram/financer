package sqlite

import (
	"database/sql"

	"github.com/mohan9182/financer/internal/repo"
	tim "github.com/mohan9182/financer/internal/time"
)

// LLMSettingsRepo implements repo.LLMSettingsRepo over SQLite.
type LLMSettingsRepo struct {
	db *sql.DB
}

func NewLLMSettingsRepo(db *sql.DB) *LLMSettingsRepo {
	return &LLMSettingsRepo{db: db}
}

func (r *LLMSettingsRepo) Get(userID string) (*repo.LLMSettingsRow, error) {
	var s repo.LLMSettingsRow
	var enabled int
	err := r.db.QueryRow(
		`SELECT base_url, model, api_key_enc, timeout_secs, enabled, min_samples,
		        failure_threshold, cooldown_minutes, auto_poll_throttle_minutes, updated_at
		 FROM user_llm_settings WHERE user_id = ?`,
		userID,
	).Scan(
		&s.BaseURL, &s.Model, &s.APIKeyEnc, &s.TimeoutSecs, &enabled, &s.MinSamples,
		&s.FailureThreshold, &s.CooldownMinutes, &s.AutoPollThrottleMinutes, &s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.UserID = userID
	s.Enabled = enabled != 0
	return &s, nil
}

func (r *LLMSettingsRepo) Upsert(s *repo.LLMSettingsRow) error {
	s.UpdatedAt = tim.NowGoTS()
	_, err := r.db.Exec(
		`INSERT INTO user_llm_settings
			(user_id, base_url, model, api_key_enc, timeout_secs, enabled,
			 min_samples, failure_threshold, cooldown_minutes, auto_poll_throttle_minutes, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET
			base_url = excluded.base_url,
			model = excluded.model,
			api_key_enc = excluded.api_key_enc,
			timeout_secs = excluded.timeout_secs,
			enabled = excluded.enabled,
			min_samples = excluded.min_samples,
			failure_threshold = excluded.failure_threshold,
			cooldown_minutes = excluded.cooldown_minutes,
			auto_poll_throttle_minutes = excluded.auto_poll_throttle_minutes,
			updated_at = excluded.updated_at`,
		s.UserID, s.BaseURL, s.Model, s.APIKeyEnc, s.TimeoutSecs, boolToInt(s.Enabled),
		s.MinSamples, s.FailureThreshold, s.CooldownMinutes, s.AutoPollThrottleMinutes, s.UpdatedAt,
	)
	return err
}
