// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	api "github.com/BZYA-Community/WebsiteCore/auto/api/v1"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/chain"
	"github.com/BZYA-Community/WebsiteCore/pkg/utils"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/alimy/tryst/cfg"
	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid/v5"
	"github.com/sirupsen/logrus"
)

// courseVideoPrefix 课程视频对象键前缀: LocalOSS对/attachment/路径强制签名校验, 课程视频沿用之
const courseVideoPrefix = "attachment/course/"

// 课程视频签名播放地址有效期: 浏览器播放过程会持续发起range请求, TTL需覆盖长视频
const courseVideoSignExpireSec = 3600

type courseLooseSrv struct {
	api.UnimplementedCourseLooseServant
	*base.DaoServant

	oss core.ObjectStorageService
}

type coursePrivSrv struct {
	api.UnimplementedCoursePrivServant
	*base.DaoServant

	oss core.ObjectStorageService
}

type courseAdminSrv struct {
	api.UnimplementedCourseAdminServant
	*base.DaoServant

	oss core.ObjectStorageService
}

func (s *courseLooseSrv) Chain() gin.HandlersChain {
	return gin.HandlersChain{chain.JwtLoose()}
}

func (s *coursePrivSrv) Chain() gin.HandlersChain {
	return gin.HandlersChain{chain.JWT(), chain.Priv()}
}

func (s *courseAdminSrv) Chain() gin.HandlersChain {
	return gin.HandlersChain{chain.JWT(), chain.Admin()}
}

// ===== 公开读取 =====

func (s *courseLooseSrv) CourseGroups(req *web.CourseGroupsReq) (*web.CourseGroupsResp, error) {
	groups, err := s.Ds.ListCourseGroups()
	if err != nil {
		logrus.Errorf("Ds.ListCourseGroups err: %s", err)
		return nil, web.ErrGetCourseGroupsFailed
	}
	return &web.CourseGroupsResp{Groups: groups}, nil
}

func (s *courseLooseSrv) CourseList(req *web.CourseListReq) (*web.CourseListResp, error) {
	limit, offset := req.PageSize, (req.Page-1)*req.PageSize
	keyword := strings.TrimSpace(req.Keyword)
	courses, total, err := s.Ds.ListCourses(req.GroupID, keyword, offset, limit)
	if err != nil {
		logrus.Errorf("Ds.ListCourses err: %s", err)
		return nil, web.ErrGetCourseListFailed
	}
	formated, err := s.formatCourses(courses)
	if err != nil {
		return nil, web.ErrGetCourseListFailed
	}
	return (*web.CourseListResp)(base.PageRespFrom(formated, req.Page, req.PageSize, total)), nil
}

func (s *courseLooseSrv) CourseDetail(req *web.CourseDetailReq) (*web.CourseDetailResp, error) {
	course, err := s.Ds.GetCourseByID(req.ID)
	if err != nil || course.Model == nil || course.ID <= 0 {
		return nil, web.ErrCourseNotExist
	}
	formated, err := s.formatCourses([]*ms.Course{course})
	if err != nil || len(formated) == 0 {
		return nil, web.ErrCourseNotExist
	}
	return &web.CourseDetailResp{Course: formated[0]}, nil
}

// formatCourses 批量填充老师信息与分组名(老师已删除时用幽灵占位)
func (s *courseLooseSrv) formatCourses(courses []*ms.Course) ([]*ms.CourseFormated, error) {
	teacherIds := make([]int64, 0, len(courses))
	for _, c := range courses {
		teacherIds = append(teacherIds, c.TeacherID)
	}
	users, err := s.Ds.GetUsersByIDs(teacherIds)
	if err != nil {
		logrus.Errorf("Ds.GetUsersByIDs err: %s", err)
		return nil, err
	}
	userMap := make(map[int64]*ms.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}
	groupName := make(map[int64]string)
	if groups, err := s.Ds.ListCourseGroups(); err == nil {
		for _, g := range groups {
			groupName[g.ID] = g.Name
		}
	}
	res := make([]*ms.CourseFormated, 0, len(courses))
	for _, c := range courses {
		item := c.Format()
		item.GroupName = groupName[c.GroupID]
		if teacher, ok := userMap[c.TeacherID]; ok {
			item.Teacher = teacher.Format()
		} else {
			item.Teacher = ms.GhostUserFormated
		}
		res = append(res, item)
	}
	return res, nil
}

func (s *courseLooseSrv) CourseComments(req *web.CourseCommentsReq) (*web.CourseCommentsResp, error) {
	course, err := s.Ds.GetCourseByID(req.ID)
	if err != nil || course.Model == nil || course.ID <= 0 {
		return nil, web.ErrCourseNotExist
	}
	// 评论可见范围与帖子评论同口径: 游客仅过审/用户过审+本人/审核员全部
	var viewerId int64
	viewerIsAuditor := false
	if req.User != nil {
		viewerId = req.User.ID
		viewerIsAuditor = req.User.IsAdmin || req.User.HasRole(ms.RoleAuditor)
	}
	limit, offset := req.PageSize, (req.Page-1)*req.PageSize
	comments, total, err := s.Ds.GetCourseComments(req.ID, viewerId, viewerIsAuditor, limit, offset)
	if err != nil {
		logrus.Errorf("Ds.GetCourseComments err: %s", err)
		return nil, web.ErrGetCourseCommentsFailed
	}
	commentIds := make([]int64, 0, len(comments))
	userIds := make([]int64, 0, len(comments))
	for _, comment := range comments {
		commentIds = append(commentIds, comment.ID)
		userIds = append(userIds, comment.UserID)
	}
	contentsMap := make(map[int64][]*ms.CourseCommentContent, len(commentIds))
	if contents, err := s.Ds.GetCourseCommentContentsByIDs(commentIds); err == nil {
		for _, content := range contents {
			contentsMap[content.CommentID] = append(contentsMap[content.CommentID], content)
		}
	}
	repliesMap := make(map[int64][]*ms.CourseCommentReplyFormated, len(commentIds))
	if replies, err := s.Ds.GetCourseCommentRepliesByID(commentIds, viewerId, viewerIsAuditor); err == nil {
		for _, reply := range replies {
			repliesMap[reply.CommentID] = append(repliesMap[reply.CommentID], reply)
		}
	}
	users, err := s.Ds.GetUsersByIDs(userIds)
	if err != nil {
		logrus.Errorf("Ds.GetUsersByIDs err: %s", err)
		return nil, web.ErrGetCourseCommentsFailed
	}
	userMap := make(map[int64]*ms.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}
	items := make([]*ms.CourseCommentFormated, 0, len(comments))
	for _, comment := range comments {
		item := comment.Format()
		item.Contents = contentsMap[comment.ID]
		if replies, ok := repliesMap[comment.ID]; ok {
			item.Replies = replies
		}
		if user, ok := userMap[comment.UserID]; ok {
			item.User = user.Format()
		} else {
			item.User = ms.GhostUserFormated
		}
		items = append(items, item)
	}
	return (*web.CourseCommentsResp)(base.PageRespFrom(items, req.Page, req.PageSize, total)), nil
}

func (s *courseLooseSrv) CourseVideo(req *web.CourseVideoReq) (*web.CourseVideoResp, error) {
	course, err := s.Ds.GetCourseByID(req.ID)
	if err != nil || course.Model == nil || course.ID <= 0 {
		return nil, web.ErrCourseNotExist
	}
	objectKey := s.oss.ObjectKey(course.VideoURL)
	signedURL, err := s.oss.SignURL(objectKey, courseVideoSignExpireSec)
	if err != nil {
		logrus.Errorf("oss.SignURL err: %s", err)
		return nil, web.ErrCourseVideoInvalid
	}
	return &web.CourseVideoResp{SignedURL: signedURL}, nil
}

func (s *courseLooseSrv) CoursePlay(req *web.CoursePlayReq) (*web.CoursePlayResp, error) {
	if _, err := s.Ds.GetCourseByID(req.ID); err != nil {
		return nil, web.ErrCourseNotExist
	}
	count, err := s.Ds.IncrCoursePlayCount(req.ID)
	if err != nil {
		logrus.Errorf("Ds.IncrCoursePlayCount err: %s", err)
		return nil, web.ErrGetCourseListFailed
	}
	return &web.CoursePlayResp{PlayCount: count}, nil
}

// ===== 评论(登录用户, 审核口径同帖子评论) =====

func (s *coursePrivSrv) CreateCourseComment(req *web.CreateCourseCommentReq) (*web.CreateCourseCommentResp, error) {
	course, err := s.Ds.GetCourseByID(req.CourseID)
	if err != nil || course.Model == nil || course.ID <= 0 {
		return nil, web.ErrCourseNotExist
	}
	if course.CommentCount >= conf.AppSetting.MaxCommentCount {
		return nil, web.ErrMaxCommentCount
	}
	user, err := s.Ds.GetUserByID(req.Uid)
	if err != nil {
		logrus.Errorf("Ds.GetUserByID err: %s", err)
		return nil, web.ErrCreateCourseCommentFailed
	}
	mediaContents, err := persistMediaContents(s.oss, req.Contents)
	if err != nil {
		return nil, web.ErrCreateCourseCommentFailed
	}
	// 审核开关: 无管理角色的用户评论需先过审 课程评论计数延迟到过审时生效
	needAudit := conf.AuditSetting.Enabled && !user.HasAnyRole()
	comment := &ms.CourseComment{
		CourseID: course.ID,
		UserID:   req.Uid,
		IP:       req.ClientIP,
		IPLoc:    utils.GetIPLoc(req.ClientIP),
	}
	if needAudit {
		comment.AuditStatus = ms.PostAuditPending
	} else {
		comment.AuditStatus = ms.PostAuditApproved
	}
	comment, err = s.Ds.CreateCourseComment(comment)
	if err != nil {
		logrus.Errorf("Ds.CreateCourseComment err: %s", err)
		deleteOssObjects(s.oss, mediaContents)
		return nil, web.ErrCreateCourseCommentFailed
	}
	for _, item := range req.Contents {
		// 检查附件是否是本站资源
		if item.Type == ms.ContentTypeImage || item.Type == ms.ContentTypeVideo || item.Type == ms.ContentTypeAttachment {
			if err := s.Ds.CheckAttachment(item.Content); err != nil {
				continue
			}
		}
		commentContent := &ms.CourseCommentContent{
			CommentID: comment.ID,
			UserID:    req.Uid,
			Content:   item.Content,
			Type:      item.Type,
			Sort:      item.Sort,
		}
		if _, err = s.Ds.CreateCourseCommentContent(commentContent); err != nil {
			logrus.Errorf("Ds.CreateCourseCommentContent err: %s", err)
		}
	}
	if comment.AuditStatus == ms.PostAuditApproved {
		if err := s.Ds.AdjustCourseCommentCount(course.ID, 1); err != nil {
			logrus.Errorf("Ds.AdjustCourseCommentCount err: %s", err)
		}
	}
	return (*web.CreateCourseCommentResp)(comment.Format()), nil
}

func (s *coursePrivSrv) CreateCourseCommentReply(req *web.CreateCourseCommentReplyReq) (*web.CreateCourseCommentReplyResp, error) {
	comment, err := s.Ds.GetCourseCommentByID(req.CommentID)
	if err != nil || comment.Model == nil || comment.ID <= 0 {
		return nil, web.ErrGetCourseCommentsFailed
	}
	if _, err = s.Ds.GetCourseByID(comment.CourseID); err != nil {
		return nil, web.ErrCourseNotExist
	}
	user, err := s.Ds.GetUserByID(req.Uid)
	if err != nil {
		logrus.Errorf("Ds.GetUserByID err: %s", err)
		return nil, web.ErrCreateCourseCommentFailed
	}
	atUserID := req.AtUserID
	if atUserID == req.Uid {
		atUserID = 0
	}
	if atUserID > 0 {
		// 检测目标用户是否存在
		if users, _ := s.Ds.GetUsersByIDs([]int64{atUserID}); len(users) == 0 {
			atUserID = 0
		}
	}
	// 审核开关: 无管理角色的用户回复需先过审 计数延迟到过审时生效
	needAudit := conf.AuditSetting.Enabled && !user.HasAnyRole()
	reply := &ms.CourseCommentReply{
		CommentID: req.CommentID,
		UserID:    req.Uid,
		AtUserID:  atUserID,
		Content:   req.Content,
		IP:        req.ClientIP,
		IPLoc:     utils.GetIPLoc(req.ClientIP),
	}
	if needAudit {
		reply.AuditStatus = ms.PostAuditPending
	} else {
		reply.AuditStatus = ms.PostAuditApproved
	}
	reply, err = s.Ds.CreateCourseCommentReply(reply)
	if err != nil {
		logrus.Errorf("Ds.CreateCourseCommentReply err: %s", err)
		return nil, web.ErrCreateCourseCommentFailed
	}
	if reply.AuditStatus == ms.PostAuditApproved {
		if err := s.Ds.AdjustCourseCommentCount(comment.CourseID, 1); err != nil {
			logrus.Errorf("Ds.AdjustCourseCommentCount err: %s", err)
		}
	}
	return (*web.CreateCourseCommentReplyResp)(reply.Format()), nil
}

func (s *coursePrivSrv) DeleteCourseComment(req *web.DeleteCourseCommentReq) error {
	comment, err := s.Ds.GetCourseCommentByID(req.ID)
	if err != nil || comment.Model == nil || comment.ID <= 0 {
		return web.ErrGetCourseCommentsFailed
	}
	user, err := s.Ds.GetUserByID(req.Uid)
	if err != nil {
		logrus.Errorf("Ds.GetUserByID err: %s", err)
		return web.ErrDeleteCourseCommentFailed
	}
	if comment.UserID != req.Uid && !user.IsAdmin {
		return web.ErrNoPermission
	}
	if err := s.Ds.DeleteCourseComment(comment); err != nil {
		logrus.Errorf("Ds.DeleteCourseComment err: %s", err)
		return web.ErrDeleteCourseCommentFailed
	}
	// 仅已过审评论曾计入课程评论数 删除时回减
	if comment.AuditStatus == ms.PostAuditApproved {
		if err := s.Ds.AdjustCourseCommentCount(comment.CourseID, -1); err != nil {
			logrus.Errorf("Ds.AdjustCourseCommentCount err: %s", err)
		}
	}
	return nil
}

func (s *coursePrivSrv) DeleteCourseCommentReply(req *web.DeleteCourseCommentReplyReq) error {
	reply, err := s.Ds.GetCourseCommentReplyByID(req.ID)
	if err != nil || reply.Model == nil || reply.ID <= 0 {
		return web.ErrGetCourseCommentsFailed
	}
	user, err := s.Ds.GetUserByID(req.Uid)
	if err != nil {
		logrus.Errorf("Ds.GetUserByID err: %s", err)
		return web.ErrDeleteCourseCommentFailed
	}
	if reply.UserID != req.Uid && !user.IsAdmin {
		return web.ErrNoPermission
	}
	if err := s.Ds.DeleteCourseCommentReply(reply); err != nil {
		logrus.Errorf("Ds.DeleteCourseCommentReply err: %s", err)
		return web.ErrDeleteCourseCommentFailed
	}
	// 仅已过审回复曾计入课程评论数 删除时回减(父评论reply_count已在DAO内回减)
	if reply.AuditStatus == ms.PostAuditApproved {
		if comment, err := s.Ds.GetCourseCommentByID(reply.CommentID); err == nil {
			if err := s.Ds.AdjustCourseCommentCount(comment.CourseID, -1); err != nil {
				logrus.Errorf("Ds.AdjustCourseCommentCount err: %s", err)
			}
		}
	}
	return nil
}

// ===== 管理(管理员/运维) =====

func (s *courseAdminSrv) CreateCourseGroup(req *web.CourseGroupReq) (*web.CourseGroupResp, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > 64 {
		return nil, xerror.InvalidParams.WithDetails("分组名称为1~64字")
	}
	group, err := s.Ds.CreateCourseGroup(&ms.CourseGroup{
		Name: name,
		Sort: req.Sort,
	})
	if err != nil {
		logrus.Errorf("Ds.CreateCourseGroup err: %s", err)
		return nil, web.ErrCreateCourseFailed
	}
	return (*web.CourseGroupResp)(group.Format(0)), nil
}

func (s *courseAdminSrv) UpdateCourseGroup(req *web.UpdateCourseGroupReq) error {
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > 64 {
		return xerror.InvalidParams.WithDetails("分组名称为1~64字")
	}
	if _, err := s.Ds.GetCourseGroupByID(req.ID); err != nil {
		return web.ErrCourseGroupNotExist
	}
	return s.Ds.UpdateCourseGroup(&ms.CourseGroup{
		Model: &ms.Model{ID: req.ID},
		Name:  name,
		Sort:  req.Sort,
	})
}

func (s *courseAdminSrv) DeleteCourseGroup(req *web.DeleteCourseGroupReq) error {
	if _, err := s.Ds.GetCourseGroupByID(req.ID); err != nil {
		return web.ErrCourseGroupNotExist
	}
	if count, err := s.Ds.CountCoursesByGroup(req.ID); err != nil {
		logrus.Errorf("Ds.CountCoursesByGroup err: %s", err)
		return web.ErrDeleteCourseFailed
	} else if count > 0 {
		return web.ErrCourseGroupNotEmpty
	}
	return s.Ds.DeleteCourseGroup(req.ID)
}

func (s *courseAdminSrv) CreateCourse(req *web.CreateCourseReq) (*web.CreateCourseResp, error) {
	course, videoKey, err := s.buildCourse(req.GroupID, req.TeacherID, req.Title, req.Intro, req.Video, req.Cover, nil)
	if err != nil {
		return nil, err
	}
	course, err = s.Ds.CreateCourse(course)
	if err != nil {
		logrus.Errorf("Ds.CreateCourse err: %s", err)
		// 落库失败清理已上传的视频对象
		s.oss.DeleteObjects([]string{videoKey})
		return nil, web.ErrCreateCourseFailed
	}
	return (*web.CreateCourseResp)(course.Format()), nil
}

func (s *courseAdminSrv) UpdateCourse(req *web.UpdateCourseReq) error {
	course, err := s.Ds.GetCourseByID(req.ID)
	if err != nil || course.Model == nil || course.ID <= 0 {
		return web.ErrCourseNotExist
	}
	oldVideoKey := s.oss.ObjectKey(course.VideoURL)
	oldCoverKey := s.oss.ObjectKey(course.Cover)
	video, cover := req.Video, req.Cover
	if video == "" {
		video = course.VideoURL
	}
	if cover == "" {
		cover = course.Cover
	}
	updated, _, err := s.buildCourse(req.GroupID, req.TeacherID, req.Title, req.Intro, video, cover, course)
	if err != nil {
		return err
	}
	updated.Model = course.Model
	if err := s.Ds.UpdateCourse(updated); err != nil {
		logrus.Errorf("Ds.UpdateCourse err: %s", err)
		return web.ErrUpdateCourseFailed
	}
	// 更换视频/封面后清理旧对象(宽松处理)
	var staleKeys []string
	if newKey := s.oss.ObjectKey(updated.VideoURL); oldVideoKey != "" && newKey != oldVideoKey {
		staleKeys = append(staleKeys, oldVideoKey)
	}
	if newKey := s.oss.ObjectKey(updated.Cover); oldCoverKey != "" && newKey != oldCoverKey {
		staleKeys = append(staleKeys, oldCoverKey)
	}
	if len(staleKeys) > 0 {
		s.oss.DeleteObjects(staleKeys)
	}
	return nil
}

// buildCourse 校验并组装课程(分组/老师存在性, 视频/封面OSS对象有效性)
// 返回组装好的课程与视频对象键(供调用方在落库失败时清理)
func (s *courseAdminSrv) buildCourse(groupID, teacherID int64, title, intro, video, cover string, exist *ms.Course) (*ms.Course, string, error) {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > 128 {
		return nil, "", xerror.InvalidParams.WithDetails("课程标题为1~128字")
	}
	if utf8.RuneCountInString(intro) > 2000 {
		return nil, "", xerror.InvalidParams.WithDetails("课程简介最长2000字")
	}
	if _, err := s.Ds.GetCourseGroupByID(groupID); err != nil {
		return nil, "", web.ErrCourseGroupNotExist
	}
	if _, err := s.Ds.GetUserByID(teacherID); err != nil {
		return nil, "", web.ErrCourseTeacherInvalid
	}
	// 视频: 直传模式传对象键, 代理模式传完整URL, 统一归一化为键校验
	videoKey := video
	if strings.Contains(video, "://") {
		videoKey = s.oss.ObjectKey(video)
	}
	if !strings.HasPrefix(videoKey, courseVideoPrefix) {
		return nil, "", web.ErrCourseVideoInvalid
	}
	if exist == nil || exist.VideoURL == "" || s.oss.ObjectKey(exist.VideoURL) != videoKey {
		// 新上传的对象: 校验存在并转持久
		if ok, err := s.oss.IsObjectExist(videoKey); err != nil || !ok {
			return nil, "", web.ErrCourseVideoInvalid
		}
		if err := s.oss.PersistObject(videoKey); err != nil {
			logrus.Warnf("oss.PersistObject(%s) err: %s", videoKey, err)
		}
	}
	course := &ms.Course{
		GroupID:   groupID,
		TeacherID: teacherID,
		Title:     title,
		Intro:     intro,
		VideoURL:  s.oss.ObjectURL(videoKey),
		Cover:     strings.TrimSpace(cover),
	}
	return course, videoKey, nil
}

func (s *courseAdminSrv) DeleteCourse(req *web.DeleteCourseReq) error {
	course, err := s.Ds.GetCourseByID(req.ID)
	if err != nil || course.Model == nil || course.ID <= 0 {
		return web.ErrCourseNotExist
	}
	if err := s.Ds.DeleteCourse(course); err != nil {
		logrus.Errorf("Ds.DeleteCourse err: %s", err)
		return web.ErrDeleteCourseFailed
	}
	// 清理OSS对象(宽松处理 不影响删除结果)
	keys := []string{}
	if key := s.oss.ObjectKey(course.VideoURL); key != "" {
		keys = append(keys, key)
	}
	if key := s.oss.ObjectKey(course.Cover); key != "" {
		keys = append(keys, key)
	}
	if len(keys) > 0 {
		s.oss.DeleteObjects(keys)
	}
	// 审计日志: post_id列复用为课程id, action前缀course_区分
	if req.User != nil {
		if err := s.Ds.CreateAuditLog(&ms.AuditLog{
			PostID:     course.ID,
			OperatorID: req.User.ID,
			Action:     "course_delete",
		}); err != nil {
			logrus.Errorf("Ds.CreateAuditLog err: %s", err)
		}
	}
	return nil
}

// CourseUploadCredential 视频上传凭证: AliOSS返回PostObject直传签名, 其他OSS返回proxy(走后端中转)
func (s *courseAdminSrv) CourseUploadCredential(req *web.CourseUploadCredentialReq) (*web.CourseUploadCredentialResp, error) {
	var fileExt string
	switch strings.ToLower(req.Ext) {
	case ".mp4", "mp4":
		fileExt = ".mp4"
	case ".mov", "mov":
		fileExt = ".mov"
	default:
		return nil, web.ErrCourseVideoInvalid.WithDetails("课程视频仅允许 mp4/mov 类型")
	}
	if !cfg.If("AliOSS") {
		return &web.CourseUploadCredentialResp{Mode: "proxy"}, nil
	}
	objectKey := courseVideoPrefix + time.Now().Format("200601") + "/" + uuid.Must(uuid.NewV4()).String() + fileExt
	// Ali OSS PostObject policy: 标准表单直传签名(密钥不出服务端)
	expiration := time.Now().Add(10 * time.Minute).UTC().Format("2006-01-02T15:04:05.000Z")
	policyBytes, err := json.Marshal(map[string]any{
		"expiration": expiration,
		"conditions": []any{
			map[string]string{"bucket": conf.AliOSSSetting.Bucket},
			[]string{"eq", "$key", objectKey},
			[]any{"content-length-range", 1, 524288000}, // 500MB
			[]string{"eq", "$success_action_status", "200"},
		},
	})
	if err != nil {
		logrus.Errorf("marshal oss policy err: %s", err)
		return nil, web.ErrCourseUploadCredentialFailed
	}
	policy := base64.StdEncoding.EncodeToString(policyBytes)
	mac := hmac.New(sha1.New, []byte(conf.AliOSSSetting.AccessKeySecret))
	mac.Write([]byte(policy))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	// 直传地址用Endpoint而非Domain(Domain可能是CDN/自定义域名)
	host := fmt.Sprintf("https://%s.%s", conf.AliOSSSetting.Bucket, conf.AliOSSSetting.Endpoint)
	return &web.CourseUploadCredentialResp{
		Mode:        "direct",
		Host:        host,
		AccessKeyID: conf.AliOSSSetting.AccessKeyID,
		Policy:      policy,
		Signature:   signature,
		Key:         objectKey,
		Expire:      time.Now().Add(10 * time.Minute).Unix(),
	}, nil
}

// UploadCourseVideo 代理模式上传课程视频(非AliOSS或直传不可用时)
func (s *courseAdminSrv) UploadCourseVideo(req *web.UploadCourseVideoReq) (*web.UploadCourseVideoResp, error) {
	defer req.File.Close()
	objectKey := courseVideoPrefix + time.Now().Format("200601") + "/" + uuid.Must(uuid.NewV4()).String() + req.FileExt
	objectURL, err := s.oss.PutObject(objectKey, req.File, req.FileSize, req.ContentType, false)
	if err != nil {
		logrus.Errorf("oss.PutObject err: %s", err)
		return nil, web.ErrFileUploadFailed
	}
	return &web.UploadCourseVideoResp{VideoURL: objectURL}, nil
}

func newCourseLooseSrv(s *base.DaoServant, oss core.ObjectStorageService) api.CourseLoose {
	return &courseLooseSrv{
		DaoServant: s,
		oss:        oss,
	}
}

func newCoursePrivSrv(s *base.DaoServant, oss core.ObjectStorageService) api.CoursePriv {
	return &coursePrivSrv{
		DaoServant: s,
		oss:        oss,
	}
}

func newCourseAdminSrv(s *base.DaoServant, oss core.ObjectStorageService) api.CourseAdmin {
	return &courseAdminSrv{
		DaoServant: s,
		oss:        oss,
	}
}
