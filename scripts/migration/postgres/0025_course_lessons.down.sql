UPDATE p_course AS course
SET video_url = lesson.video_url
FROM (
    SELECT DISTINCT ON (course_id) course_id, video_url
    FROM p_course_lesson
    WHERE is_del = 0 AND video_url <> ''
    ORDER BY course_id, sort, id
) AS lesson
WHERE course.id = lesson.course_id;

DROP TABLE IF EXISTS p_course_lesson_attachment;
DROP TABLE IF EXISTS p_course_lesson;
DROP INDEX IF EXISTS uq_course_title_in_group;
DROP INDEX IF EXISTS uq_course_group_name;
