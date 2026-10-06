-- Optional courses: category hierarchy, lessons, verified upload references.
ALTER TABLE p_course_group ADD COLUMN parent_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE p_course_group ADD CONSTRAINT course_group_not_own_parent CHECK (parent_id >= 0 AND parent_id <> id);
CREATE INDEX idx_course_group_parent ON p_course_group (parent_id, sort, id);
ALTER TABLE p_course ADD COLUMN teacher_intro TEXT NOT NULL DEFAULT '';

ALTER TABLE p_attachment ADD COLUMN purpose VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE p_attachment ADD COLUMN mime_type VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE p_attachment ADD COLUMN verified BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE p_course_lesson (
    id BIGSERIAL PRIMARY KEY,
    course_id BIGINT NOT NULL REFERENCES p_course(id) ON DELETE CASCADE,
    title VARCHAR(128) NOT NULL,
    intro TEXT NOT NULL DEFAULT '',
    sort INT NOT NULL DEFAULT 0,
    created_on BIGINT NOT NULL DEFAULT 0,
    modified_on BIGINT NOT NULL DEFAULT 0,
    deleted_on BIGINT NOT NULL DEFAULT 0,
    is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_course_lesson_order ON p_course_lesson (course_id, is_del, sort, id);

CREATE TABLE p_course_lesson_attachment (
    id BIGSERIAL PRIMARY KEY,
    lesson_id BIGINT NOT NULL REFERENCES p_course_lesson(id) ON DELETE CASCADE,
    attachment_id BIGINT NOT NULL REFERENCES p_attachment(id),
    name VARCHAR(255) NOT NULL,
    kind VARCHAR(16) NOT NULL CHECK (kind IN ('attachment', 'resource')),
    sort INT NOT NULL DEFAULT 0,
    UNIQUE (lesson_id, attachment_id)
);
CREATE INDEX idx_course_lesson_attachment_order ON p_course_lesson_attachment (lesson_id, sort, id);
CREATE INDEX idx_course_lesson_attachment_upload ON p_course_lesson_attachment (attachment_id);

CREATE TABLE p_operation_log (
    id BIGSERIAL PRIMARY KEY,
    actor_id BIGINT NOT NULL,
    action VARCHAR(32) NOT NULL,
    entity VARCHAR(32) NOT NULL,
    entity_id BIGINT NOT NULL DEFAULT 0,
    before_json TEXT NOT NULL DEFAULT 'null',
    after_json TEXT NOT NULL DEFAULT 'null',
    created_on BIGINT NOT NULL
);
CREATE INDEX idx_operation_log_entity ON p_operation_log (entity, entity_id, id);
CREATE INDEX idx_operation_log_actor ON p_operation_log (actor_id, id);
