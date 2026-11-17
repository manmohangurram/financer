-- LLM samples and sender templates look up one sender's recent mail; the queue
-- index (user_id, created_at) made that scan the whole mailbox.
CREATE INDEX idx_mail_messages_user_sender ON mail_messages(user_id, from_addr, created_at DESC);
