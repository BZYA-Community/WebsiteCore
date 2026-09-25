-- 评论/回复审核 + 昵称变更审核
-- audit_status: 0待审核 1已通过 2未通过 (存量数据默认1保持可见)
ALTER TABLE `p_comment` ADD COLUMN `audit_status` SMALLINT NOT NULL DEFAULT 1;
ALTER TABLE `p_comment_reply` ADD COLUMN `audit_status` SMALLINT NOT NULL DEFAULT 1;
-- 昵称变更暂存字段: 提交后先存此处 审核通过才写入 nickname
ALTER TABLE `p_user` ADD COLUMN `pending_nickname` VARCHAR(32) NOT NULL DEFAULT '';
