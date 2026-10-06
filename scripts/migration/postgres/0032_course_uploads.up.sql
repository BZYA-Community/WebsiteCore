ALTER TABLE p_attachment ADD COLUMN name VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE p_attachment ADD COLUMN upload_expires_on BIGINT NOT NULL DEFAULT 0;
CREATE INDEX idx_attachment_pending_upload ON p_attachment(user_id, upload_expires_on) WHERE verified = FALSE AND is_del = 0;
