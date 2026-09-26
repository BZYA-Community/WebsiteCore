// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package conf

import (
	"database/sql"
	"fmt"
	"sync"
)

var (
	_sqldb    *sql.DB
	_sqldbErr error
	_onceSql  sync.Once
)

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

// SqlDB 返回共享的 *sql.DB，首次调用时创建；失败时返回 error 而不是杀进程。
func SqlDB() (*sql.DB, error) {
	_onceSql.Do(func() {
		var err error
		if _, _sqldb, err = newSqlDB(); err != nil {
			_sqldbErr = fmt.Errorf("new sql db failed: %w", err)
		}
	})
	return _sqldb, _sqldbErr
}

// CloseDB close databse to prevent data missing
func CloseDB() {
	closeGormDB()
}

func newSqlDB() (driver string, db *sql.DB, err error) {
	driver = "pgx"
	db, err = sql.Open(driver, PostgresSetting.Dsn())
	return
}
