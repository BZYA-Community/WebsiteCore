// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package conf

const (
	TableAnouncement          = "anouncement"
	TableAnouncementContent   = "anouncement_content"
	TableAttachment           = "attachment"
	TableCaptcha              = "captcha"
	TableComment              = "comment"
	TableCourse               = "course"
	TableCourseGroup          = "course_group"
	TableCourseComment        = "course_comment"
	TableCourseCommentContent = "course_comment_content"
	TableCourseCommentReply   = "course_comment_reply"
	TableCommentMetric        = "comment_metric"
	TableCommentContent       = "comment_content"
	TableCommentReply         = "comment_reply"
	TableFollowing            = "following"
	TableMessage              = "message"
	TablePost                 = "post"
	TablePostMetric           = "post_metric"
	TablePostByComment        = "post_by_comment"
	TablePostByMedia          = "post_by_media"
	TablePostCollection       = "post_collection"
	TablePostContent          = "post_content"
	TablePostStar             = "post_star"
	TableTag                  = "tag"
	TableSiteSettings         = "site_settings"
	TableUser                 = "user"
	TableUserRelation         = "user_relation"
	TableUserMetric           = "user_metric"
)

type TableNameMap map[string]string

// CloseDB closes the application's database connection pool.
func CloseDB() {
	closeGormDB()
}
