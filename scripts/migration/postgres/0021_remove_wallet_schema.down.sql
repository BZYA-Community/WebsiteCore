-- 回滚: 恢复钱包/付费相关结构(仅供回退 不恢复历史数据)
ALTER TABLE "p_user" ADD COLUMN "balance" BIGINT NOT NULL DEFAULT 0;
DROP TABLE IF EXISTS "p_post_attachment_bill";
CREATE TABLE "p_post_attachment_bill" (
	id BIGSERIAL PRIMARY KEY,
	post_id BIGINT NOT NULL DEFAULT 0,
	user_id BIGINT NOT NULL DEFAULT 0,
	paid_amount BIGINT NOT NULL DEFAULT 0,
	created_on BIGINT NOT NULL DEFAULT 0,
	modified_on BIGINT NOT NULL DEFAULT 0,
	deleted_on BIGINT NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_post_attachment_bill_post_id ON "p_post_attachment_bill" USING btree (post_id);
CREATE INDEX idx_post_attachment_bill_user_id ON "p_post_attachment_bill" USING btree (user_id);
DROP TABLE IF EXISTS "p_wallet_recharge";
CREATE TABLE "p_wallet_recharge" (
	id BIGSERIAL PRIMARY KEY,
	user_id BIGINT NOT NULL DEFAULT 0,
	amount BIGINT NOT NULL DEFAULT 0,
	trade_no VARCHAR(64) NOT NULL DEFAULT '',
	trade_status VARCHAR(32) NOT NULL DEFAULT '',
	created_on BIGINT NOT NULL DEFAULT 0,
	modified_on BIGINT NOT NULL DEFAULT 0,
	deleted_on BIGINT NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_wallet_recharge_user_id ON "p_wallet_recharge" USING btree (user_id);
CREATE INDEX idx_wallet_recharge_trade_no ON "p_wallet_recharge" USING btree (trade_no);
CREATE INDEX idx_wallet_recharge_trade_status ON "p_wallet_recharge" USING btree (trade_status);
DROP TABLE IF EXISTS "p_wallet_statement";
CREATE TABLE "p_wallet_statement" (
	id BIGSERIAL PRIMARY KEY,
	user_id BIGINT NOT NULL DEFAULT 0,
	change_amount BIGINT NOT NULL DEFAULT 0,
	balance_snapshot BIGINT NOT NULL DEFAULT 0,
	reason VARCHAR(255) NOT NULL DEFAULT '',
	post_id BIGINT NOT NULL DEFAULT 0,
	created_on BIGINT NOT NULL DEFAULT 0,
	modified_on BIGINT NOT NULL DEFAULT 0,
	deleted_on BIGINT NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_wallet_statement_user_id ON "p_wallet_statement" USING btree (user_id);
