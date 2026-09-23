-- 用户管理角色与内容审核系统
ALTER TABLE p_user ADD COLUMN roles TEXT NOT NULL DEFAULT '';

UPDATE p_user SET roles = CASE WHEN username = 'kiana' THEN 'operator' WHEN is_admin != 0 THEN 'admin' ELSE '' END;

ALTER TABLE p_post ADD COLUMN audit_status SMALLINT NOT NULL DEFAULT 1;

CREATE TABLE p_audit_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	post_id INTEGER NOT NULL DEFAULT 0,
	operator_id INTEGER NOT NULL DEFAULT 0,
	action VARCHAR(32) NOT NULL DEFAULT '',
	old_status SMALLINT NOT NULL DEFAULT 0,
	new_status SMALLINT NOT NULL DEFAULT 0,
	reason TEXT NOT NULL DEFAULT '',
	created_on INTEGER NOT NULL DEFAULT 0,
	modified_on INTEGER NOT NULL DEFAULT 0,
	deleted_on INTEGER NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_audit_log_post_id ON p_audit_log (post_id);
CREATE INDEX idx_audit_log_created_on ON p_audit_log (created_on);

CREATE TABLE p_user_role_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL DEFAULT 0,
	operator_id INTEGER NOT NULL DEFAULT 0,
	old_roles TEXT NOT NULL DEFAULT '',
	new_roles TEXT NOT NULL DEFAULT '',
	action VARCHAR(32) NOT NULL DEFAULT '',
	created_on INTEGER NOT NULL DEFAULT 0,
	modified_on INTEGER NOT NULL DEFAULT 0,
	deleted_on INTEGER NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_user_role_log_user_id ON p_user_role_log (user_id);
