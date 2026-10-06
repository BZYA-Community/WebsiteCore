CREATE TABLE p_review_task (
    id BIGSERIAL PRIMARY KEY,
    kind VARCHAR(32) NOT NULL CHECK (kind IN ('post','comment','reply','course_question','course_answer','nickname','avatar')),
    target_id BIGINT NOT NULL,
    author_id BIGINT NOT NULL,
    revision BIGINT NOT NULL DEFAULT 1,
    state VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','completed','cancelled')),
    snapshot TEXT NOT NULL DEFAULT '',
    assignee_id BIGINT NOT NULL DEFAULT 0,
    previous_assignee_id BIGINT NOT NULL DEFAULT 0,
    assigned_on BIGINT NOT NULL DEFAULT 0,
    deadline_on BIGINT NOT NULL DEFAULT 0,
    completed_on BIGINT NOT NULL DEFAULT 0,
    created_on BIGINT NOT NULL DEFAULT 0,
    modified_on BIGINT NOT NULL DEFAULT 0,
    UNIQUE (kind, target_id)
);
CREATE INDEX idx_review_task_queue ON p_review_task (state, assignee_id, deadline_on);
CREATE TABLE p_review_task_event (
    id BIGSERIAL PRIMARY KEY,
    task_id BIGINT NOT NULL REFERENCES p_review_task(id),
    revision BIGINT NOT NULL,
    event VARCHAR(32) NOT NULL,
    actor_id BIGINT NOT NULL DEFAULT 0,
    from_assignee_id BIGINT NOT NULL DEFAULT 0,
    to_assignee_id BIGINT NOT NULL DEFAULT 0,
    reason VARCHAR(255) NOT NULL DEFAULT '',
    created_on BIGINT NOT NULL
);
CREATE INDEX idx_review_task_event_task ON p_review_task_event (task_id, id);
CREATE INDEX idx_review_task_event_misses ON p_review_task_event (from_assignee_id) WHERE event = 'timeout';

-- Existing pending content is enrolled once during migration. New submissions
-- enqueue inside their content transaction, never through a scan of partial writes.
INSERT INTO p_review_task (kind,target_id,author_id,snapshot,created_on,modified_on)
SELECT 'post',id,user_id,'',created_on,modified_on FROM p_post WHERE is_del=0 AND audit_status=0 AND visibility<>0
UNION ALL SELECT 'comment',id,user_id,'',created_on,modified_on FROM p_comment WHERE is_del=0 AND audit_status=0
UNION ALL SELECT 'reply',id,user_id,'',created_on,modified_on FROM p_comment_reply WHERE is_del=0 AND audit_status=0
UNION ALL SELECT 'course_question',id,user_id,'',created_on,modified_on FROM p_course_comment WHERE is_del=0 AND audit_status=0
UNION ALL SELECT 'course_answer',id,user_id,'',created_on,modified_on FROM p_course_comment_reply WHERE is_del=0 AND audit_status=0
UNION ALL SELECT 'nickname',id,id,pending_nickname,created_on,modified_on FROM p_user WHERE is_del=0 AND pending_nickname<>''
UNION ALL SELECT 'avatar',id,id,pending_avatar,created_on,modified_on FROM p_user WHERE is_del=0 AND pending_avatar<>'';
