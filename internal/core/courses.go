// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package core

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

// CourseService 课程检索服务(游客可读; 评论可见性按viewer审核口径)
type CourseService interface {
	GetCourseByID(id int64) (*ms.Course, error)
	GetCourseGroupByID(id int64) (*ms.CourseGroup, error)
	// ListCourseGroups 全量分组(按sort/id排序, 各组附带课程数)
	ListCourseGroups() ([]*ms.CourseGroupFormated, error)
	// ListCourses 课程列表: groupId>0按分组过滤; keyword非空按标题/简介模糊匹配(课程页独立搜索)
	ListCourses(groupId int64, keyword string, offset, limit int) ([]*ms.Course, int64, error)
	// GetCourseComments 评论可见范围与帖子评论同口径:
	//   游客: 仅已过审; 登录用户: 已过审+本人; 审核员/管理员: 全部
	GetCourseComments(courseId, viewerId int64, viewerIsAuditor bool, limit, offset int) ([]*ms.CourseComment, int64, error)
	GetCourseCommentByID(id int64) (*ms.CourseComment, error)
	GetCourseCommentReplyByID(id int64) (*ms.CourseCommentReply, error)
	// GetCourseCommentRepliesByReplyIDs 批量按回复id取课程回复(供列表场景一次取回避免逐行查询)
	// 软删除的不返回 查询失败返回错误 缺失的id不在返回中
	GetCourseCommentRepliesByReplyIDs(ids []int64) ([]*ms.CourseCommentReply, error)
	GetCourseCommentContentsByIDs(ids []int64) ([]*ms.CourseCommentContent, error)
	GetCourseCommentRepliesByID(ids []int64, viewerId int64, viewerIsAuditor bool) ([]*ms.CourseCommentReplyFormated, error)
}

// CourseManageService 课程管理服务(管理员/运维)
type CourseManageService interface {
	CreateCourseGroup(g *ms.CourseGroup) (*ms.CourseGroup, error)
	UpdateCourseGroup(g *ms.CourseGroup) error
	// DeleteCourseGroup 分组硬删除(调用方需先校验分组下无课程)
	DeleteCourseGroup(id int64) error
	CountCoursesByGroup(groupId int64) (int64, error)
	CreateCourse(c *ms.Course) (*ms.Course, error)
	UpdateCourse(c *ms.Course) error
	// DeleteCourse 课程硬删除: 同事务硬删其评论/回复/内容
	DeleteCourse(course *ms.Course) error
	// IncrCoursePlayCount 播放量原子+1(每次播放计一次, 不去重), 返回最新值
	IncrCoursePlayCount(id int64) (int64, error)
	CreateCourseComment(c *ms.CourseComment) (*ms.CourseComment, error)
	CreateCourseCommentContent(c *ms.CourseCommentContent) (*ms.CourseCommentContent, error)
	CreateCourseCommentReply(r *ms.CourseCommentReply) (*ms.CourseCommentReply, error)
	DeleteCourseComment(c *ms.CourseComment) error
	DeleteCourseCommentReply(r *ms.CourseCommentReply) error
	// UpdateCourseCommentAuditStatus 更新课程评论审核状态, 返回旧状态供上层联动
	UpdateCourseCommentAuditStatus(id int64, status int) (int, error)
	// UpdateCourseCommentReplyAuditStatus 更新课程回复审核状态, 联动父评论reply_count, 返回旧状态
	UpdateCourseCommentReplyAuditStatus(id int64, status int) (int, error)
	// AdjustCourseCommentCount 课程评论数增减(仅已过审评论计入)
	AdjustCourseCommentCount(courseId int64, delta int) error
}
