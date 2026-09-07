-- API key scope: "read" (default) or "read_write".
ALTER TABLE user_keys ADD COLUMN scope TEXT NOT NULL DEFAULT 'read';
