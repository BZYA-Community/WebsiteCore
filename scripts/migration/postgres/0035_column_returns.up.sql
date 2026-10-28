ALTER TABLE p_column ADD COLUMN admin_deleted_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE p_column ADD COLUMN admin_deleted_on BIGINT NOT NULL DEFAULT 0;
ALTER TABLE p_column ADD COLUMN admin_delete_reason VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE p_column ADD COLUMN recreated_from_id BIGINT NOT NULL DEFAULT 0;
CREATE INDEX idx_column_author_returns ON p_column(user_id, admin_deleted_on DESC, id DESC)
    WHERE is_del = 1 AND admin_deleted_by > 0 AND kind = 'article';
ALTER TABLE p_message ADD COLUMN column_returned BOOLEAN NOT NULL DEFAULT FALSE;
