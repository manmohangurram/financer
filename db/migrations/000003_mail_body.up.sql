-- Display-ready form of each stored message: the decoded sender name and a
-- parsed plain-text body (headers and quoted history removed). The raw message
-- is still kept in mail_messages.raw_text for template extraction and samples.
ALTER TABLE mail_messages ADD COLUMN body_text TEXT NOT NULL DEFAULT '';
ALTER TABLE mail_messages ADD COLUMN from_name TEXT NOT NULL DEFAULT '';
