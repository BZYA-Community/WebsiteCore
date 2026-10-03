-- 邮箱验证：用户邮箱与邮件验证码
ALTER TABLE "p_user" ADD COLUMN "email" VARCHAR(254) NOT NULL DEFAULT '';
CREATE UNIQUE INDEX "idx_user_email_not_empty" ON "p_user" ("email") WHERE "email" <> '';

ALTER TABLE "p_captcha" ADD COLUMN "email" VARCHAR(254) NOT NULL DEFAULT '';
CREATE INDEX "idx_captcha_email" ON "p_captcha" ("email");
