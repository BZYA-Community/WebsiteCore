-- 回滚: 恢复钱包/付费相关结构(仅供回退 不恢复历史数据)
ALTER TABLE `p_user` ADD COLUMN `balance` BIGINT NOT NULL DEFAULT 0 COMMENT '用户余额（分）';
DROP TABLE IF EXISTS `p_post_attachment_bill`;
CREATE TABLE `p_post_attachment_bill` (
	`id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '购买记录ID',
	`post_id` BIGINT NOT NULL DEFAULT '0' COMMENT 'POST ID',
	`user_id` BIGINT NOT NULL DEFAULT '0' COMMENT '用户ID',
	`paid_amount` BIGINT NOT NULL DEFAULT '0' COMMENT '支付金额',
	`created_on` BIGINT NOT NULL DEFAULT '0' COMMENT '创建时间',
	`modified_on` BIGINT NOT NULL DEFAULT '0' COMMENT '修改时间',
	`deleted_on` BIGINT NOT NULL DEFAULT '0' COMMENT '删除时间',
	`is_del` tinyint NOT NULL DEFAULT '0' COMMENT '是否删除 0 为未删除、1 为已删除',
	PRIMARY KEY (`id`) USING BTREE,
	KEY `idx_post_attachment_bill_post_id` (`post_id`) USING BTREE,
	KEY `idx_post_attachment_bill_user_id` (`user_id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='付费附件购买记录';
DROP TABLE IF EXISTS `p_wallet_recharge`;
CREATE TABLE `p_wallet_recharge` (
	`id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '充值ID',
	`user_id` BIGINT NOT NULL DEFAULT '0' COMMENT '用户ID',
	`amount` BIGINT NOT NULL DEFAULT '0' COMMENT '充值金额',
	`trade_no` varchar(64) NOT NULL DEFAULT '' COMMENT '支付宝订单号',
	`trade_status` varchar(32) NOT NULL DEFAULT '' COMMENT '交易状态',
	`created_on` BIGINT NOT NULL DEFAULT '0' COMMENT '创建时间',
	`modified_on` BIGINT NOT NULL DEFAULT '0' COMMENT '修改时间',
	`deleted_on` BIGINT NOT NULL DEFAULT '0' COMMENT '删除时间',
	`is_del` tinyint NOT NULL DEFAULT '0' COMMENT '是否删除 0 为未删除、1 为已删除',
	PRIMARY KEY (`id`) USING BTREE,
	KEY `idx_wallet_recharge_user_id` (`user_id`) USING BTREE,
	KEY `idx_wallet_recharge_trade_no` (`trade_no`) USING BTREE,
	KEY `idx_wallet_recharge_trade_status` (`trade_status`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='钱包充值';
DROP TABLE IF EXISTS `p_wallet_statement`;
CREATE TABLE `p_wallet_statement` (
	`id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '账单ID',
	`user_id` BIGINT NOT NULL DEFAULT '0' COMMENT '用户ID',
	`change_amount` BIGINT NOT NULL DEFAULT '0' COMMENT '变动金额',
	`balance_snapshot` BIGINT NOT NULL DEFAULT '0' COMMENT '资金快照',
	`reason` varchar(255) NOT NULL COMMENT '变动原因',
	`post_id` BIGINT NOT NULL DEFAULT '0' COMMENT '关联动态',
	`created_on` BIGINT NOT NULL DEFAULT '0' COMMENT '创建时间',
	`modified_on` BIGINT NOT NULL DEFAULT '0' COMMENT '修改时间',
	`deleted_on` BIGINT NOT NULL DEFAULT '0' COMMENT '删除时间',
	`is_del` tinyint NOT NULL DEFAULT '0' COMMENT '是否删除 0 为未删除、1 为已删除',
	PRIMARY KEY (`id`) USING BTREE,
	KEY `idx_wallet_statement_user_id` (`user_id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='钱包流水';
