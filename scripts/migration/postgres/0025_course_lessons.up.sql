-- Preserve all legacy rows and disambiguate only pre-existing conflicts.
WITH ranked AS (
    SELECT id, row_number() OVER (PARTITION BY lower(trim(name)) ORDER BY id) AS rn
    FROM p_course_group WHERE is_del = 0
)
UPDATE p_course_group AS target
SET name = trim(target.name) || ' (' || target.id || ')'
FROM ranked WHERE target.id = ranked.id AND ranked.rn > 1;

WITH ranked AS (
    SELECT id, row_number() OVER (PARTITION BY group_id, lower(trim(title)) ORDER BY id) AS rn
    FROM p_course WHERE is_del = 0
)
UPDATE p_course AS target
SET title = trim(target.title) || ' (' || target.id || ')'
FROM ranked WHERE target.id = ranked.id AND ranked.rn > 1;

CREATE UNIQUE INDEX uq_course_group_name
    ON p_course_group (lower(trim(name))) WHERE is_del = 0;
CREATE UNIQUE INDEX uq_course_title_in_group
    ON p_course (group_id, lower(trim(title))) WHERE is_del = 0;

CREATE TABLE p_course_lesson (
	id BIGSERIAL PRIMARY KEY,
	course_id BIGINT NOT NULL,
	title VARCHAR(128) NOT NULL DEFAULT '',
	summary TEXT NOT NULL DEFAULT '',
	video_url VARCHAR(255) NOT NULL DEFAULT '',
	sort INT NOT NULL DEFAULT 0,
	created_on BIGINT NOT NULL DEFAULT 0,
	modified_on BIGINT NOT NULL DEFAULT 0,
	deleted_on BIGINT NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_course_lesson_course ON p_course_lesson (course_id, is_del, sort, id);
CREATE UNIQUE INDEX uq_course_lesson_title
    ON p_course_lesson (course_id, lower(trim(title))) WHERE is_del = 0;

CREATE TABLE p_course_lesson_attachment (
	id BIGSERIAL PRIMARY KEY,
	lesson_id BIGINT NOT NULL,
	name VARCHAR(255) NOT NULL DEFAULT '',
	url VARCHAR(500) NOT NULL DEFAULT '',
	sort INT NOT NULL DEFAULT 0,
	created_on BIGINT NOT NULL DEFAULT 0,
	modified_on BIGINT NOT NULL DEFAULT 0,
	deleted_on BIGINT NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_course_lesson_attachment_lesson
    ON p_course_lesson_attachment (lesson_id, is_del, sort, id);

INSERT INTO p_course_lesson
    (course_id, title, summary, video_url, sort, created_on, modified_on, deleted_on, is_del)
SELECT id, title, '', video_url, 0, created_on, modified_on, 0, 0
FROM p_course WHERE is_del = 0;
