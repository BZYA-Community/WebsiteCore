-- Add an assignable reviewer identity without granting administrative powers.
-- Do not replace an existing custom group or its permissions.
WITH added AS (
    INSERT INTO p_identity_group (key, name, description, builtin)
    VALUES ('reviewer', '审核员', 'Review assigned content; no user or identity administration.', TRUE)
    ON CONFLICT (key) DO NOTHING
    RETURNING id
)
INSERT INTO p_identity_group_permission (group_id, permission)
SELECT id, 'content.review' FROM added;

CREATE INDEX idx_user_created_on_active ON p_user (created_on, id) WHERE is_del = 0;
