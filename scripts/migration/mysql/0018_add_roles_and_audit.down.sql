DROP TABLE `p_user_role_log`;
DROP TABLE `p_audit_log`;
ALTER TABLE `p_post` DROP COLUMN `audit_status`;
ALTER TABLE `p_user` DROP COLUMN `roles`;
