-- 用户管理角色与内容审核系统
-- roles: 逗号分隔的管理角色 operator/admin/auditor/mentor (基础身份游客/道友由手机号绑定状态推导，不落库)
ALTER TABLE p_user ADD COLUMN roles VARCHAR(255) NOT NULL DEFAULT '';

-- 存量管理员映射: kiana -> 运维(operator) 其余 -> 管理员(admin)
UPDATE p_user SET roles = CASE WHEN username = 'kiana' THEN 'operator' WHEN is_admin THEN 'admin' ELSE '' END;

-- audit_status: 0待审核 1已通过 2未通过 (删除复用软删 is_del; 存量帖默认已通过保持可见)
ALTER TABLE p_post ADD COLUMN audit_status SMALLINT NOT NULL DEFAULT 1;

-- 审核操作日志
CREATE TABLE p_audit_log (
	id BIGSERIAL PRIMARY KEY,
	post_id BIGINT NOT NULL DEFAULT 0,
	operator_id BIGINT NOT NULL DEFAULT 0,
	action VARCHAR(32) NOT NULL DEFAULT '',
	old_status SMALLINT NOT NULL DEFAULT 0,
	new_status SMALLINT NOT NULL DEFAULT 0,
	reason VARCHAR(255) NOT NULL DEFAULT '',
	created_on BIGINT NOT NULL DEFAULT 0,
	modified_on BIGINT NOT NULL DEFAULT 0,
	deleted_on BIGINT NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_audit_log_post_id ON p_audit_log USING btree (post_id);
CREATE INDEX idx_audit_log_created_on ON p_audit_log USING btree (created_on);

-- 用户角色变更日志
CREATE TABLE p_user_role_log (
	id BIGSERIAL PRIMARY KEY,
	user_id BIGINT NOT NULL DEFAULT 0,
	operator_id BIGINT NOT NULL DEFAULT 0,
	old_roles VARCHAR(255) NOT NULL DEFAULT '',
	new_roles VARCHAR(255) NOT NULL DEFAULT '',
	action VARCHAR(32) NOT NULL DEFAULT '',
	created_on BIGINT NOT NULL DEFAULT 0,
	modified_on BIGINT NOT NULL DEFAULT 0,
	deleted_on BIGINT NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_user_role_log_user_id ON p_user_role_log USING btree (user_id);
