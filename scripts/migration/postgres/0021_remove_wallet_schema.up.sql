-- 移除钱包/付费遗留的表与列(公益项目无收费能力)
ALTER TABLE "p_user" DROP COLUMN IF EXISTS "balance";
DROP TABLE IF EXISTS "p_post_attachment_bill";
DROP TABLE IF EXISTS "p_wallet_recharge";
DROP TABLE IF EXISTS "p_wallet_statement";
