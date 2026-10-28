DELETE FROM p_identity_group_permission WHERE permission = 'column.publish';
DELETE FROM p_review_task_event WHERE task_id IN (SELECT id FROM p_review_task WHERE kind IN ('column', 'column_comment'));
DELETE FROM p_review_task WHERE kind IN ('column', 'column_comment');
ALTER TABLE p_message DROP COLUMN column_id;
ALTER TABLE p_audit_log DROP COLUMN column_id;
DROP TABLE p_column_reaction;
DROP TABLE p_column_comment;
DROP TABLE p_column_entry;
DROP TABLE p_column_submission;
DROP TABLE p_column;

ALTER TABLE p_review_task DROP CONSTRAINT p_review_task_kind_check;
ALTER TABLE p_review_task ADD CONSTRAINT p_review_task_kind_check CHECK (kind IN ('post','comment','reply','course_question','course_answer','nickname','avatar'));
