DROP INDEX IF EXISTS idx_attachment_pending_upload;
ALTER TABLE p_attachment DROP COLUMN IF EXISTS upload_expires_on;
ALTER TABLE p_attachment DROP COLUMN IF EXISTS name;
