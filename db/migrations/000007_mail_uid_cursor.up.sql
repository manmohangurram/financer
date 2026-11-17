-- Incremental polling: the highest UID seen per mailbox, so a poll after a restart
-- picks up every new message instead of only the last N days.
ALTER TABLE mail_accounts ADD COLUMN last_uid INTEGER NOT NULL DEFAULT 0;
