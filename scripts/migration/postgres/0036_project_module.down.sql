DELETE FROM p_post_content AS content
USING p_project AS project
WHERE content.post_id = project.post_id
	AND content.type = 6
	AND content.content = 'project:' || project.id;

DROP TABLE IF EXISTS p_project_follow;
DROP TABLE IF EXISTS p_project_member;
DROP TABLE IF EXISTS p_project_tag;
DROP TABLE IF EXISTS p_project_image;
DROP TABLE IF EXISTS p_project;
