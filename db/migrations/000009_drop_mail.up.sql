-- Email ingest was removed from the product. Drop the tables the feature owned:
-- linked mailboxes, queued messages, mail rules, learned templates and the
-- per-user LLM settings that existed only to parse mail. Transactions that mail
-- once created stay in the ledger.
DROP TABLE IF EXISTS mail_messages;
DROP TABLE IF EXISTS mail_rules;
DROP TABLE IF EXISTS mail_accounts;
DROP TABLE IF EXISTS mail_templates;
DROP TABLE IF EXISTS user_llm_settings;
