// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"mime/multipart"

	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/gin-gonic/gin"
)

// ===== 课程公开读取(游客可用) =====

type CourseGroupsReq struct {
	BaseInfo `form:"-" binding:"-"`
}

type CourseGroupsResp struct {
	Groups []*ms.CourseGroupFormated `json:"groups"`
}

// CourseListReq 课程列表: group_id>0按分组过滤; keyword非空按标题/简介模糊搜索(课程页独立搜索)
type CourseListReq struct {
	BaseInfo `form:"-" binding:"-"`
	GroupID  int64  `form:"group_id"`
	Keyword  string `form:"keyword"`
	Page     int    `form:"-" binding:"-"`
	PageSize int    `form:"-" binding:"-"`
}

func (r *CourseListReq) SetPageInfo(page, pageSize int) {
	r.Page, r.PageSize = page, pageSize
}

type CourseListResp base.PageResp

type CourseDetailReq struct {
	BaseInfo `form:"-" binding:"-"`
	ID       int64 `form:"id" binding:"required"`
}

type CourseDetailResp struct {
	Course *ms.CourseFormated `json:"course"`
}

type CourseCommentsReq struct {
	BaseInfo `form:"-" binding:"-"`
	ID       int64 `form:"id" binding:"required"`
	Page     int   `form:"-" binding:"-"`
	PageSize int   `form:"-" binding:"-"`
}

func (r *CourseCommentsReq) SetPageInfo(page, pageSize int) {
	r.Page, r.PageSize = page, pageSize
}

type CourseCommentsResp base.PageResp

// CourseVideoReq 获取课程视频签名播放地址
type CourseVideoReq struct {
	BaseInfo `form:"-" binding:"-"`
	ID       int64 `form:"id" binding:"required"`
}

type CourseVideoResp struct {
	SignedURL string `json:"signed_url"`
}

// CoursePlayReq 播放计数(每次播放+1, 不去重)
type CoursePlayReq struct {
	BaseInfo `json:"-" binding:"-"`
	ID       int64 `json:"id" binding:"required"`
}

type CoursePlayResp struct {
	PlayCount int64 `json:"play_count"`
}

// ===== 课程评论(登录用户, 审核口径同帖子评论) =====

type CreateCourseCommentReq struct {
	SimpleInfo `json:"-" binding:"-"`
	CourseID   int64              `json:"course_id" binding:"required"`
	Contents   []*PostContentItem `json:"contents" binding:"required"`
	Users      []string           `json:"users" binding:"required"`
	ClientIP   string             `json:"-" binding:"-"`
}

type CreateCourseCommentResp = ms.CourseCommentFormated

type CreateCourseCommentReplyReq struct {
	SimpleInfo `json:"-" binding:"-"`
	CommentID  int64  `json:"comment_id" binding:"required"`
	AtUserID   int64  `json:"at_user_id"`
	Content    string `json:"content" binding:"required"`
	ClientIP   string `json:"-" binding:"-"`
}

type CreateCourseCommentReplyResp = ms.CourseCommentReplyFormated

type DeleteCourseCommentReq struct {
	SimpleInfo `json:"-" binding:"-"`
	ID         int64 `json:"id" binding:"required"`
}

type DeleteCourseCommentReplyReq struct {
	SimpleInfo `json:"-" binding:"-"`
	ID         int64 `json:"id" binding:"required"`
}

// ===== 课程管理(管理员/运维) =====

type CourseGroupReq struct {
	BaseInfo `json:"-" binding:"-"`
	Name     string `json:"name" binding:"required"`
	Sort     int    `json:"sort"`
}

type CourseGroupResp = ms.CourseGroupFormated

type UpdateCourseGroupReq struct {
	BaseInfo `json:"-" binding:"-"`
	ID       int64  `json:"id" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Sort     int    `json:"sort"`
}

type DeleteCourseGroupReq struct {
	BaseInfo `json:"-" binding:"-"`
	ID       int64 `json:"id" binding:"required"`
}

// CreateCourseReq 创建课程: Video 为OSS对象键(直传)或完整URL(代理上传返回), Cover为图片URL
type CreateCourseReq struct {
	BaseInfo  `json:"-" binding:"-"`
	GroupID   int64  `json:"group_id" binding:"required"`
	TeacherID int64  `json:"teacher_id" binding:"required"`
	Title     string `json:"title" binding:"required"`
	Intro     string `json:"intro"`
	Video     string `json:"video" binding:"required"`
	Cover     string `json:"cover"`
}

type CreateCourseResp = ms.CourseFormated

// UpdateCourseReq 更新课程: Video/Cover 为空字符串表示不更换
type UpdateCourseReq struct {
	BaseInfo  `json:"-" binding:"-"`
	ID        int64  `json:"id" binding:"required"`
	GroupID   int64  `json:"group_id" binding:"required"`
	TeacherID int64  `json:"teacher_id" binding:"required"`
	Title     string `json:"title" binding:"required"`
	Intro     string `json:"intro"`
	Video     string `json:"video"`
	Cover     string `json:"cover"`
}

type DeleteCourseReq struct {
	BaseInfo `json:"-" binding:"-"`
	ID       int64 `json:"id" binding:"required"`
}

// CourseUploadCredentialReq 获取视频上传凭证: AliOSS返回直传policy, 其他OSS返回proxy模式
type CourseUploadCredentialReq struct {
	BaseInfo `form:"-" binding:"-"`
	Ext      string `form:"ext" binding:"required"`
}

type CourseUploadCredentialResp struct {
	Mode string `json:"mode"` // direct=浏览器直传OSS / proxy=后端中转上传
	// 以下仅 direct 模式返回
	Host        string `json:"host,omitempty"`
	AccessKeyID string `json:"access_key_id,omitempty"`
	Policy      string `json:"policy,omitempty"`
	Signature   string `json:"signature,omitempty"`
	Key         string `json:"key,omitempty"`
	Expire      int64  `json:"expire,omitempty"`
}

// UploadCourseVideoReq 代理模式视频上传(multipart)
type UploadCourseVideoReq struct {
	SimpleInfo  `json:"-" binding:"-"`
	File        multipart.File
	FileSize    int64
	FileExt     string
	ContentType string
}

type UploadCourseVideoResp struct {
	VideoURL string `json:"video_url"`
}

// maxCourseVideoBodySize 课程视频上传请求体上限: 文件500MB + 开销余量(#29)
const maxCourseVideoBodySize int64 = 500*1024*1024 + uploadBodyHeadroom

func (r *UploadCourseVideoReq) Bind(c *gin.Context) (xerr error) {
	userId, exist := base.UserIdFrom(c)
	if !exist {
		return xerror.UnauthorizedAuthNotExist
	}
	// 读取前先限制请求体大小, 超限在读取阶段即拒绝(#29)
	applyUploadBodyLimit(c, maxCourseVideoBodySize)
	if err := parseUploadForm(c, ErrFileInvalidSize.WithDetails("课程视频最大允许500MB")); err != nil {
		return err
	}
	file, fileHeader, err := c.Request.FormFile("file")
	if err != nil {
		return ErrFileUploadFailed
	}
	defer func() {
		if xerr != nil {
			file.Close()
		}
	}()
	// 课程视频上限500MB(代理上传受HTTP读写超时约束, 生产环境建议Ali直传)
	if fileHeader.Size > 1024*1024*500 {
		return ErrFileInvalidSize.WithDetails("课程视频最大允许500MB")
	}
	contentType := fileHeader.Header.Get("Content-Type")
	var fileExt string
	switch contentType {
	case "video/mp4":
		fileExt = ".mp4"
	case "video/quicktime":
		fileExt = ".mov"
	default:
		return ErrFileInvalidExt.WithDetails("课程视频仅允许 mp4/mov 类型")
	}
	r.SimpleInfo = SimpleInfo{Uid: userId}
	r.File, r.FileSize, r.FileExt, r.ContentType = file, fileHeader.Size, fileExt, contentType
	return nil
}
