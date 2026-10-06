-- Identity membership and permission grants replace role-name authorization.
-- Existing content, accounts and legacy fields are retained. Configure Operator
-- before deploying: old roles are deliberately not promoted into new privileges.
ALTER TABLE p_user ADD COLUMN is_operator BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE p_identity_group (
    id BIGSERIAL PRIMARY KEY,
    key VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description VARCHAR(500) NOT NULL DEFAULT '',
    builtin BOOLEAN NOT NULL DEFAULT FALSE,
    CONSTRAINT identity_group_no_operator CHECK (key <> 'operator')
);
CREATE TABLE p_identity_group_permission (
    group_id BIGINT NOT NULL REFERENCES p_identity_group(id) ON DELETE CASCADE,
    permission VARCHAR(64) NOT NULL,
    PRIMARY KEY (group_id, permission)
);
CREATE TABLE p_user_identity_group (
    user_id BIGINT NOT NULL REFERENCES p_user(id) ON DELETE CASCADE,
    group_id BIGINT NOT NULL REFERENCES p_identity_group(id) ON DELETE RESTRICT,
    PRIMARY KEY (user_id, group_id)
);
CREATE INDEX idx_user_identity_group_group ON p_user_identity_group(group_id);
CREATE TABLE p_identity_operation_log (
    id BIGSERIAL PRIMARY KEY,
    actor_id BIGINT NOT NULL,
    action VARCHAR(32) NOT NULL,
    group_id BIGINT NOT NULL DEFAULT 0,
    user_id BIGINT NOT NULL DEFAULT 0,
    before_json TEXT NOT NULL,
    after_json TEXT NOT NULL,
    created_on BIGINT NOT NULL
);
CREATE INDEX idx_identity_operation_log_actor ON p_identity_operation_log(actor_id);

INSERT INTO p_identity_group (key, name, description, builtin) VALUES
    ('guest', '游客', 'Automatic identity for anonymous and unverified visitors.', TRUE),
    ('member', '道友', 'Automatic identity after contact verification.', TRUE),
    ('admin', '管理身份', 'Community administration; publication still requires review.', TRUE);

INSERT INTO p_identity_group_permission (group_id, permission)
SELECT id, permission FROM p_identity_group CROSS JOIN
    (VALUES ('post.view'), ('course.catalog'), ('profile.edit')) AS defaults(permission)
WHERE key = 'guest';

INSERT INTO p_identity_group_permission (group_id, permission)
SELECT id, permission FROM p_identity_group CROSS JOIN
    (VALUES ('post.view'), ('course.catalog'), ('course.view'), ('post.create'),
     ('comment.create'), ('content.upload'), ('message.initiate'), ('message.reply'),
     ('profile.edit'), ('community.interact')) AS defaults(permission)
WHERE key = 'member';

INSERT INTO p_identity_group_permission (group_id, permission)
SELECT id, permission FROM p_identity_group CROSS JOIN
    (VALUES ('post.view'), ('post.create'), ('comment.create'), ('content.upload'),
     ('content.review'), ('content.manage'), ('content.view_private'), ('user.manage'),
     ('identity.manage'), ('course.catalog'), ('course.view'), ('course.manage'),
     ('course.manage_own'), ('course.upload'), ('site.manage'), ('message.initiate'),
     ('message.reply'), ('profile.edit'), ('community.interact'), ('audit.view_all')) AS defaults(permission)
WHERE key = 'admin';
