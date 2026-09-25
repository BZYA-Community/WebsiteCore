-- SQLite >= 3.35 支持 DROP COLUMN
ALTER TABLE `p_comment` DROP COLUMN `audit_status`;
ALTER TABLE `p_comment_reply` DROP COLUMN `audit_status`;
ALTER TABLE `p_user` DROP COLUMN `pending_nickname`;
