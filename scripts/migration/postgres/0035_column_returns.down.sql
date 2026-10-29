ALTER TABLE p_message DROP COLUMN column_returned;
DROP INDEX idx_column_author_returns;
ALTER TABLE p_column DROP COLUMN recreated_from_id;
ALTER TABLE p_column DROP COLUMN admin_delete_reason;
ALTER TABLE p_column DROP COLUMN admin_deleted_on;
ALTER TABLE p_column DROP COLUMN admin_deleted_by;
