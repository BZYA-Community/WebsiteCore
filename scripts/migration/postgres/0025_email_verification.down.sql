DROP INDEX IF EXISTS "idx_captcha_email";
ALTER TABLE "p_captcha" DROP COLUMN IF EXISTS "email";

DROP INDEX IF EXISTS "idx_user_email_not_empty";
ALTER TABLE "p_user" DROP COLUMN IF EXISTS "email";
