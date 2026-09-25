-- 回滚: 恢复钱包/付费相关结构(仅供回退 不恢复历史数据)
ALTER TABLE "p_user" ADD COLUMN "balance" integer NOT NULL DEFAULT 0;
DROP TABLE IF EXISTS "p_post_attachment_bill";
CREATE TABLE "p_post_attachment_bill" (
  "id" integer NOT NULL,
  "post_id" integer NOT NULL,
  "user_id" integer NOT NULL,
  "paid_amount" integer NOT NULL,
  "created_on" integer NOT NULL,
  "modified_on" integer NOT NULL,
  "deleted_on" integer NOT NULL,
  "is_del" integer NOT NULL,
  PRIMARY KEY ("id")
);
DROP TABLE IF EXISTS "p_wallet_recharge";
CREATE TABLE "p_wallet_recharge" (
  "id" integer NOT NULL,
  "user_id" integer NOT NULL,
  "amount" integer NOT NULL,
  "trade_no" text NOT NULL,
  "trade_status" text NOT NULL,
  "created_on" integer NOT NULL,
  "modified_on" integer NOT NULL,
  "deleted_on" integer NOT NULL,
  "is_del" integer NOT NULL,
  PRIMARY KEY ("id")
);
DROP TABLE IF EXISTS "p_wallet_statement";
CREATE TABLE "p_wallet_statement" (
  "id" integer NOT NULL,
  "user_id" integer NOT NULL,
  "change_amount" integer NOT NULL,
  "balance_snapshot" integer NOT NULL,
  "reason" text NOT NULL,
  "post_id" integer NOT NULL,
  "created_on" integer NOT NULL,
  "modified_on" integer NOT NULL,
  "deleted_on" integer NOT NULL,
  "is_del" integer NOT NULL,
  PRIMARY KEY ("id")
);
