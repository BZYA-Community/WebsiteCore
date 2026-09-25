-- 课程模块: 分组/课程/课程评论(楼中楼, 审核口径同帖子评论)
-- 课程无状态流转: 创建即上线, 删除为硬删除(不复用 is_del 软删)

-- 课程分组
CREATE TABLE p_course_group (
	id BIGSERIAL PRIMARY KEY,
	name VARCHAR(64) NOT NULL DEFAULT '',
	sort INT NOT NULL DEFAULT 0,
	created_on BIGINT NOT NULL DEFAULT 0,
	modified_on BIGINT NOT NULL DEFAULT 0,
	deleted_on BIGINT NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_course_group_sort ON p_course_group USING btree (is_del, sort, id);

-- 课程: 标题/简介由管理员编写, 封面自动取自视频截帧, teacher_id 关联 p_user
CREATE TABLE p_course (
	id BIGSERIAL PRIMARY KEY,
	group_id BIGINT NOT NULL DEFAULT 0,
	teacher_id BIGINT NOT NULL DEFAULT 0,
	title VARCHAR(128) NOT NULL DEFAULT '',
	intro TEXT NOT NULL DEFAULT '',
	video_url VARCHAR(255) NOT NULL DEFAULT '',
	cover VARCHAR(255) NOT NULL DEFAULT '',
	play_count BIGINT NOT NULL DEFAULT 0,
	comment_count BIGINT NOT NULL DEFAULT 0,
	created_on BIGINT NOT NULL DEFAULT 0,
	modified_on BIGINT NOT NULL DEFAULT 0,
	deleted_on BIGINT NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_course_group_id ON p_course USING btree (is_del, group_id, id);
CREATE INDEX idx_course_teacher_id ON p_course USING btree (teacher_id);

-- 课程评论: audit_status 0待审核 1已通过 2未通过
CREATE TABLE p_course_comment (
	id BIGSERIAL PRIMARY KEY,
	course_id BIGINT NOT NULL DEFAULT 0,
	user_id BIGINT NOT NULL DEFAULT 0,
	ip VARCHAR(64) NOT NULL DEFAULT '',
	ip_loc VARCHAR(64) NOT NULL DEFAULT '',
	reply_count INT NOT NULL DEFAULT 0,
	audit_status SMALLINT NOT NULL DEFAULT 0,
	created_on BIGINT NOT NULL DEFAULT 0,
	modified_on BIGINT NOT NULL DEFAULT 0,
	deleted_on BIGINT NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_course_comment_course ON p_course_comment USING btree (is_del, course_id, audit_status, id);
CREATE INDEX idx_course_comment_audit ON p_course_comment USING btree (is_del, audit_status, created_on);

-- 课程评论内容(分块, 与帖子评论内容同构)
CREATE TABLE p_course_comment_content (
	id BIGSERIAL PRIMARY KEY,
	comment_id BIGINT NOT NULL DEFAULT 0,
	user_id BIGINT NOT NULL DEFAULT 0,
	content TEXT NOT NULL DEFAULT '',
	type SMALLINT NOT NULL DEFAULT 1,
	sort BIGINT NOT NULL DEFAULT 0,
	created_on BIGINT NOT NULL DEFAULT 0,
	modified_on BIGINT NOT NULL DEFAULT 0,
	deleted_on BIGINT NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_course_comment_content_cid ON p_course_comment_content USING btree (is_del, comment_id);

-- 课程评论回复(楼中楼)
CREATE TABLE p_course_comment_reply (
	id BIGSERIAL PRIMARY KEY,
	comment_id BIGINT NOT NULL DEFAULT 0,
	user_id BIGINT NOT NULL DEFAULT 0,
	at_user_id BIGINT NOT NULL DEFAULT 0,
	content TEXT NOT NULL DEFAULT '',
	ip VARCHAR(64) NOT NULL DEFAULT '',
	ip_loc VARCHAR(64) NOT NULL DEFAULT '',
	audit_status SMALLINT NOT NULL DEFAULT 0,
	created_on BIGINT NOT NULL DEFAULT 0,
	modified_on BIGINT NOT NULL DEFAULT 0,
	deleted_on BIGINT NOT NULL DEFAULT 0,
	is_del SMALLINT NOT NULL DEFAULT 0
);
CREATE INDEX idx_course_comment_reply_cid ON p_course_comment_reply USING btree (is_del, comment_id, audit_status);
CREATE INDEX idx_course_comment_reply_audit ON p_course_comment_reply USING btree (is_del, audit_status, created_on);
