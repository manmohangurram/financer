-- Reasoning controls for providers that expose them (DeepSeek thinking mode).
ALTER TABLE user_llm_settings ADD COLUMN reasoning_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE user_llm_settings ADD COLUMN reasoning_effort TEXT NOT NULL DEFAULT '';
