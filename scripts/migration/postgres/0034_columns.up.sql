CREATE TABLE p_column (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES p_user(id),
    kind VARCHAR(16) NOT NULL CHECK (kind IN ('article', 'collection')),
    title VARCHAR(200) NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    cover TEXT NOT NULL DEFAULT '',
    attachments JSONB NOT NULL DEFAULT '[]',
    published_on BIGINT NOT NULL DEFAULT 0,
    updated_on BIGINT NOT NULL DEFAULT 0,
    submission_id BIGINT NOT NULL DEFAULT 0,
    public_submission_id BIGINT NOT NULL DEFAULT 0,
    official BOOLEAN NOT NULL DEFAULT FALSE,
    locked BOOLEAN NOT NULL DEFAULT FALSE,
    blocked BOOLEAN NOT NULL DEFAULT FALSE,
    curated BOOLEAN NOT NULL DEFAULT FALSE,
    created_on BIGINT NOT NULL DEFAULT 0,
    modified_on BIGINT NOT NULL DEFAULT 0,
    deleted_on BIGINT NOT NULL DEFAULT 0,
    is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_column_public ON p_column(kind, published_on DESC, id DESC) WHERE is_del = 0 AND NOT blocked;
CREATE INDEX idx_column_author ON p_column(user_id, id DESC);
CREATE TABLE p_column_submission (
    id BIGSERIAL PRIMARY KEY,
    column_id BIGINT NOT NULL REFERENCES p_column(id),
    user_id BIGINT NOT NULL REFERENCES p_user(id),
    title VARCHAR(200) NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    cover TEXT NOT NULL DEFAULT '',
    attachments JSONB NOT NULL DEFAULT '[]',
    audit_status SMALLINT NOT NULL DEFAULT 0,
    reason TEXT NOT NULL DEFAULT '',
    created_on BIGINT NOT NULL DEFAULT 0,
    modified_on BIGINT NOT NULL DEFAULT 0,
    deleted_on BIGINT NOT NULL DEFAULT 0,
    is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_column_submission_parent ON p_column_submission(column_id, id DESC);
CREATE TABLE p_column_entry (
    collection_id BIGINT NOT NULL REFERENCES p_column(id),
    article_id BIGINT NOT NULL REFERENCES p_column(id),
    position INTEGER NOT NULL,
    PRIMARY KEY (collection_id, article_id)
);
CREATE INDEX idx_column_entry_order ON p_column_entry(collection_id, position);
CREATE TABLE p_column_comment (
    id BIGSERIAL PRIMARY KEY,
    column_id BIGINT NOT NULL REFERENCES p_column(id),
    parent_id BIGINT NOT NULL DEFAULT 0,
    user_id BIGINT NOT NULL REFERENCES p_user(id),
    at_user_id BIGINT NOT NULL DEFAULT 0,
    content TEXT NOT NULL,
    contents JSONB NOT NULL DEFAULT '[]',
    mentions JSONB NOT NULL DEFAULT '[]',
    ip_loc TEXT NOT NULL DEFAULT '',
    audit_status SMALLINT NOT NULL DEFAULT 0,
    reason TEXT NOT NULL DEFAULT '',
    featured BOOLEAN NOT NULL DEFAULT FALSE,
    likes BIGINT NOT NULL DEFAULT 0,
    dislikes BIGINT NOT NULL DEFAULT 0,
    created_on BIGINT NOT NULL DEFAULT 0,
    modified_on BIGINT NOT NULL DEFAULT 0,
    deleted_on BIGINT NOT NULL DEFAULT 0,
    is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_column_comment_thread ON p_column_comment(column_id, parent_id, id) WHERE is_del = 0;
CREATE TABLE p_column_reaction (
    comment_id BIGINT NOT NULL REFERENCES p_column_comment(id),
    user_id BIGINT NOT NULL REFERENCES p_user(id),
    value SMALLINT NOT NULL CHECK (value IN (-1, 0, 1)),
    PRIMARY KEY (comment_id, user_id)
);
ALTER TABLE p_message ADD COLUMN column_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE p_audit_log ADD COLUMN column_id BIGINT NOT NULL DEFAULT 0;
INSERT INTO p_identity_group_permission (group_id, permission)
SELECT id, 'column.publish' FROM p_identity_group WHERE key IN ('member', 'admin')
ON CONFLICT DO NOTHING;

ALTER TABLE p_review_task DROP CONSTRAINT p_review_task_kind_check;
ALTER TABLE p_review_task ADD CONSTRAINT p_review_task_kind_check CHECK (kind IN ('post','comment','reply','course_question','course_answer','nickname','avatar','column','column_comment'));
