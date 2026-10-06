-- Export lessons and operation logs before an explicitly requested rollback.
-- Shared upload files and legacy course videos are never removed here.
DROP TABLE p_operation_log;
DROP TABLE p_course_lesson_attachment;
DROP TABLE p_course_lesson;
ALTER TABLE p_attachment DROP COLUMN verified, DROP COLUMN mime_type, DROP COLUMN purpose;
ALTER TABLE p_course DROP COLUMN teacher_intro;
DROP INDEX idx_course_group_parent;
ALTER TABLE p_course_group DROP CONSTRAINT course_group_not_own_parent;
ALTER TABLE p_course_group DROP COLUMN parent_id;
