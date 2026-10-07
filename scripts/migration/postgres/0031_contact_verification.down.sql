DROP TABLE IF EXISTS p_contact_verification;
DROP INDEX IF EXISTS idx_user_active_phone;
DROP INDEX IF EXISTS idx_user_active_email;
ALTER TABLE p_user DROP COLUMN IF EXISTS email;
