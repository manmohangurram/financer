ALTER TABLE mail_accounts ADD COLUMN sender_allowlist TEXT NOT NULL DEFAULT '[]';
ALTER TABLE mail_messages DROP COLUMN template_id;
DROP TABLE IF EXISTS user_llm_settings;
DROP TABLE IF EXISTS mail_templates;
