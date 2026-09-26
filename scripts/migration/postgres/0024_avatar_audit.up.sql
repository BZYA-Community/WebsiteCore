-- 头像变更审核
-- 头像变更暂存字段: 提交后先存此处 审核通过才写入 avatar
ALTER TABLE "p_user" ADD COLUMN "pending_avatar" VARCHAR(255) NOT NULL DEFAULT '';
