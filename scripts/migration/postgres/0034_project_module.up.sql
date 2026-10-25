-- 独立项目模块：项目内容、项目标签、成员与关注关系均不复用帖子数据。
CREATE TABLE p_project (
	id BIGSERIAL PRIMARY KEY,
	creator_id BIGINT NOT NULL DEFAULT 0,
	post_id BIGINT NOT NULL DEFAULT 0,
	title VARCHAR(30) NOT NULL DEFAULT '',
	intro TEXT NOT NULL DEFAULT '',
	url VARCHAR(2048) NOT NULL DEFAULT '',
	follow_count BIGINT NOT NULL DEFAULT 0,
	created_on BIGINT NOT NULL DEFAULT 0,
	modified_on BIGINT NOT NULL DEFAULT 0,
	deleted_on BIGINT NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX idx_project_post ON p_project(post_id) WHERE post_id > 0;
CREATE INDEX idx_project_title ON p_project USING btree (is_del, title, id);

CREATE TABLE p_project_image (
	id BIGSERIAL PRIMARY KEY,
	project_id BIGINT NOT NULL DEFAULT 0,
	url TEXT NOT NULL DEFAULT '',
	sort BIGINT NOT NULL DEFAULT 0,
	created_on BIGINT NOT NULL DEFAULT 0,
	modified_on BIGINT NOT NULL DEFAULT 0,
	deleted_on BIGINT NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_project_image_project ON p_project_image USING btree (is_del, project_id, sort, id);

CREATE TABLE p_project_tag (
	id BIGSERIAL PRIMARY KEY,
	project_id BIGINT NOT NULL DEFAULT 0,
	name VARCHAR(32) NOT NULL DEFAULT '',
	created_on BIGINT NOT NULL DEFAULT 0,
	modified_on BIGINT NOT NULL DEFAULT 0,
	deleted_on BIGINT NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX idx_project_tag_unique ON p_project_tag(project_id, name) WHERE is_del = 0;
CREATE INDEX idx_project_tag_name ON p_project_tag USING btree (is_del, name, project_id);

CREATE TABLE p_project_member (
	id BIGSERIAL PRIMARY KEY,
	project_id BIGINT NOT NULL DEFAULT 0,
	user_id BIGINT NOT NULL DEFAULT 0,
	role VARCHAR(16) NOT NULL DEFAULT 'member',
	created_on BIGINT NOT NULL DEFAULT 0,
	modified_on BIGINT NOT NULL DEFAULT 0,
	deleted_on BIGINT NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX idx_project_member_unique ON p_project_member(project_id, user_id) WHERE is_del = 0;

CREATE TABLE p_project_follow (
	project_id BIGINT NOT NULL,
	user_id BIGINT NOT NULL,
	created_on BIGINT NOT NULL DEFAULT 0,
	PRIMARY KEY (project_id, user_id)
);
CREATE INDEX idx_project_follow_user ON p_project_follow USING btree (user_id, created_on DESC);

-- 将已有“项目”帖子回填为独立项目；保留原帖子内容以兼容历史展示。
INSERT INTO p_project (creator_id, post_id, title, intro, url, created_on, modified_on)
SELECT p.user_id,
	p.id,
	LEFT(COALESCE((SELECT pc.content FROM p_post_content pc WHERE pc.post_id = p.id AND pc.type = 1 AND pc.is_del = 0 ORDER BY pc.sort, pc.id LIMIT 1), '未命名项目'), 30),
	COALESCE((SELECT pc.content FROM p_post_content pc WHERE pc.post_id = p.id AND pc.type IN (9, 2) AND pc.is_del = 0 ORDER BY CASE WHEN pc.type = 9 THEN 0 ELSE 1 END, pc.sort, pc.id LIMIT 1), ''),
	LEFT(COALESCE((SELECT pc.content FROM p_post_content pc WHERE pc.post_id = p.id AND pc.type = 6 AND pc.is_del = 0 ORDER BY pc.sort, pc.id LIMIT 1), ''), 2048),
	p.created_on,
	p.modified_on
FROM p_post p
WHERE p.is_del = 0
	AND ('项目' = ANY(string_to_array(p.tags, ',')))
	AND NOT EXISTS (SELECT 1 FROM p_project project WHERE project.post_id = p.id);

INSERT INTO p_project_image (project_id, url, sort, created_on, modified_on)
SELECT project.id, pc.content, pc.sort, pc.created_on, pc.modified_on
FROM p_project project
JOIN p_post_content pc ON pc.post_id = project.post_id
WHERE pc.type = 3 AND pc.is_del = 0;

INSERT INTO p_project_member (project_id, user_id, role, created_on, modified_on)
SELECT id, creator_id, 'owner', created_on, modified_on FROM p_project;

INSERT INTO p_post_content (post_id, user_id, content, type, sort, created_on, modified_on, deleted_on, is_del)
SELECT project.post_id,
	project.creator_id,
	'project:' || project.id,
	6,
	COALESCE((SELECT MAX(pc.sort) + 1 FROM p_post_content pc WHERE pc.post_id = project.post_id), 100),
	project.created_on,
	project.modified_on,
	0,
	0
FROM p_project project
WHERE NOT EXISTS (
	SELECT 1 FROM p_post_content pc
	WHERE pc.post_id = project.post_id AND pc.type = 6 AND pc.content = 'project:' || project.id AND pc.is_del = 0
);
