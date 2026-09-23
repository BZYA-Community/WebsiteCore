-- 用户管理角色与内容审核系统
ALTER TABLE `p_user` ADD COLUMN `roles` VARCHAR(255) NOT NULL DEFAULT '' AFTER `is_admin`;

UPDATE `p_user` SET `roles` = CASE WHEN `username` = 'kiana' THEN 'operator' WHEN `is_admin` <> 0 THEN 'admin' ELSE '' END;

ALTER TABLE `p_post` ADD COLUMN `audit_status` SMALLINT NOT NULL DEFAULT 1;

CREATE TABLE `p_audit_log` (
	`id` BIGINT NOT NULL AUTO_INCREMENT,
	`post_id` BIGINT NOT NULL DEFAULT 0,
	`operator_id` BIGINT NOT NULL DEFAULT 0,
	`action` VARCHAR(32) NOT NULL DEFAULT '',
	`old_status` SMALLINT NOT NULL DEFAULT 0,
	`new_status` SMALLINT NOT NULL DEFAULT 0,
	`reason` VARCHAR(255) NOT NULL DEFAULT '',
	`created_on` BIGINT NOT NULL DEFAULT 0,
	`modified_on` BIGINT NOT NULL DEFAULT 0,
	`deleted_on` BIGINT NOT NULL DEFAULT 0,
	`is_del` TINYINT NOT NULL DEFAULT 0,
	PRIMARY KEY (`id`) USING BTREE,
	INDEX `idx_audit_log_post_id`(`post_id`) USING BTREE,
	INDEX `idx_audit_log_created_on`(`created_on`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='内容审核操作日志';

CREATE TABLE `p_user_role_log` (
	`id` BIGINT NOT NULL AUTO_INCREMENT,
	`user_id` BIGINT NOT NULL DEFAULT 0,
	`operator_id` BIGINT NOT NULL DEFAULT 0,
	`old_roles` VARCHAR(255) NOT NULL DEFAULT '',
	`new_roles` VARCHAR(255) NOT NULL DEFAULT '',
	`action` VARCHAR(32) NOT NULL DEFAULT '',
	`created_on` BIGINT NOT NULL DEFAULT 0,
	`modified_on` BIGINT NOT NULL DEFAULT 0,
	`deleted_on` BIGINT NOT NULL DEFAULT 0,
	`is_del` TINYINT NOT NULL DEFAULT 0,
	PRIMARY KEY (`id`) USING BTREE,
	INDEX `idx_user_role_log_user_id`(`user_id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='用户角色变更日志';
