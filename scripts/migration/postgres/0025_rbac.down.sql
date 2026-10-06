-- Export these policy/audit tables before rollback if their history is needed.
DROP TABLE p_identity_operation_log;
DROP TABLE p_user_identity_group;
DROP TABLE p_identity_group_permission;
DROP TABLE p_identity_group;
ALTER TABLE p_user DROP COLUMN is_operator;
