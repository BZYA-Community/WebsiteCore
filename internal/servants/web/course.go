// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"path"
	"strings"
	"unicode/utf8"

	"github.com/BZYA-Community/WebsiteCore/internal/model/joint"

	api "github.com/BZYA-Community/WebsiteCore/auto/api/v1"
	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/chain"
	"github.com/BZYA-Community/WebsiteCore/pkg/utils"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/gin-gonic/gin"
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
	return gin.HandlersChain{chain.JwtLoose(), chain.Authorize()}
}

func (s *coursePrivSrv) Chain() gin.HandlersChain {
	return gin.HandlersChain{chain.JWT(), chain.Authorize()}
}

func (s *courseAdminSrv) Chain() gin.HandlersChain {
	return gin.HandlersChain{chain.JWT(), chain.Authorize()}
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
	return (*web.CourseListResp)(joint.PageRespFrom(formated, req.Page, req.PageSize, total)), nil
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
		viewerIsAuditor = req.User.HasPermission("audit.view_all")
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
	return (*web.CourseCommentsResp)(joint.PageRespFrom(items, req.Page, req.PageSize, total)), nil
}

func (s *courseLooseSrv) CourseVideo(req *web.CourseVideoReq) (*web.CourseVideoResp, error) {
	if !req.User.HasPermission(authz.CourseView) {
		return nil, web.ErrNoPermission
	}
	course, err := s.Ds.GetCourseByID(req.ID)
	if err != nil || course.Model == nil || course.ID <= 0 {
		return nil, web.ErrCourseNotExist
	}
	objectKey := s.oss.ObjectKey(course.VideoURL)
	if objectKey == "" || !strings.HasPrefix(objectKey, courseVideoPrefix) {
		return nil, web.ErrCourseVideoInvalid
	}
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
	if !user.HasPermission("course.view") || !user.HasPermission("comment.create") {
		return nil, web.ErrNoPermission
	}
	if err := persistMediaContents(s.oss, req.Contents); err != nil {
		return nil, web.ErrCreateCourseCommentFailed
	}
	// Course questions are always reviewed, including teacher/operator submissions.
	comment := &ms.CourseComment{
		CourseID:    course.ID,
		UserID:      req.Uid,
		IP:          req.ClientIP,
		IPLoc:       utils.GetIPLoc(req.ClientIP),
		AuditStatus: ms.PostAuditPending,
	}
	questionContents := make([]*ms.CourseCommentContent, 0, len(req.Contents))
	for _, item := range req.Contents {
		// 检查附件是否是本站资源
		if item.Type == ms.ContentTypeImage || item.Type == ms.ContentTypeVideo || item.Type == ms.ContentTypeAttachment {
			if err := s.Ds.CheckAttachment(item.Content); err != nil {
				continue
			}
		}
		questionContents = append(questionContents, &ms.CourseCommentContent{
			UserID:  req.Uid,
			Content: item.Content,
			Type:    item.Type,
			Sort:    item.Sort,
		})
	}
	comment, err = s.Ds.CreateCourseQuestionWithContents(comment, questionContents)
	if err != nil {
		logrus.Errorf("Ds.CreateCourseQuestionWithContents err: %s", err)
		return nil, web.ErrCreateCourseCommentFailed
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
	if !user.HasPermission("course.view") || !canReplyToComment(user, comment.UserID, comment.AuditStatus) {
		return nil, web.ErrNoPermission
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
	// Answers use the same mandatory review policy as questions.
	reply := &ms.CourseCommentReply{
		CommentID:   req.CommentID,
		UserID:      req.Uid,
		AtUserID:    atUserID,
		Content:     req.Content,
		IP:          req.ClientIP,
		IPLoc:       utils.GetIPLoc(req.ClientIP),
		AuditStatus: ms.PostAuditPending,
	}
	reply, err = s.Ds.CreateCourseCommentReply(reply)
	if err != nil {
		logrus.Errorf("Ds.CreateCourseCommentReply err: %s", err)
		return nil, web.ErrCreateCourseCommentFailed
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
	if comment.UserID != req.Uid && !user.HasPermission("content.manage") {
		return web.ErrNoPermission
	}
	if err := s.Ds.DeleteCourseComment(comment); err != nil {
		logrus.Errorf("Ds.DeleteCourseComment err: %s", err)
		return web.ErrDeleteCourseCommentFailed
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
	if reply.UserID != req.Uid && !user.HasPermission("content.manage") {
		return web.ErrNoPermission
	}
	if err := s.Ds.DeleteCourseCommentReply(reply); err != nil {
		logrus.Errorf("Ds.DeleteCourseCommentReply err: %s", err)
		return web.ErrDeleteCourseCommentFailed
	}
	return nil
}

// ===== 管理(管理员/运维) =====

func canManageCourse(user *ms.User, teacherID int64) bool {
	return user != nil && (user.HasPermission("course.manage") ||
		(user.ID == teacherID && user.HasPermission("course.manage_own")))
}

func canUploadCourse(user *ms.User) bool {
	return user != nil && user.HasPermission("course.upload") &&
		(user.HasPermission("course.manage") || user.HasPermission("course.manage_own"))
}

func (s *courseAdminSrv) CreateCourseGroup(req *web.CourseGroupReq) (*web.CourseGroupResp, error) {
	if req.User == nil || !req.User.HasPermission("course.manage") {
		return nil, web.ErrNoPermission
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > 64 {
		return nil, xerror.InvalidParams.WithDetails("分组名称为1~64字")
	}
	group, err := s.Ds.CreateCourseGroup(req.User, &ms.CourseGroup{
		Name:     name,
		Sort:     req.Sort,
		ParentID: req.ParentID,
	})
	if err != nil {
		logrus.Errorf("Ds.CreateCourseGroup err: %s", err)
		return nil, courseMutationError(err, web.ErrCreateCourseFailed)
	}
	return (*web.CourseGroupResp)(group.Format(0)), nil
}

func (s *courseAdminSrv) UpdateCourseGroup(req *web.UpdateCourseGroupReq) error {
	if req.User == nil || !req.User.HasPermission("course.manage") {
		return web.ErrNoPermission
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > 64 {
		return xerror.InvalidParams.WithDetails("分组名称为1~64字")
	}
	if _, err := s.Ds.GetCourseGroupByID(req.ID); err != nil {
		return web.ErrCourseGroupNotExist
	}
	return courseMutationError(s.Ds.UpdateCourseGroup(req.User, &ms.CourseGroup{
		Model:    &ms.Model{ID: req.ID},
		Name:     name,
		Sort:     req.Sort,
		ParentID: req.ParentID,
	}), web.ErrUpdateCourseFailed)
}

func (s *courseAdminSrv) DeleteCourseGroup(req *web.DeleteCourseGroupReq) error {
	if req.User == nil || !req.User.HasPermission("course.manage") {
		return web.ErrNoPermission
	}
	if _, err := s.Ds.GetCourseGroupByID(req.ID); err != nil {
		return web.ErrCourseGroupNotExist
	}
	return courseMutationError(s.Ds.DeleteCourseGroup(req.User, req.ID), web.ErrDeleteCourseFailed)
}

func (s *courseAdminSrv) CreateCourse(req *web.CreateCourseReq) (*web.CreateCourseResp, error) {
	if req.User == nil || !canManageCourse(req.User, req.User.ID) {
		return nil, web.ErrNoPermission
	}
	teacherID := req.TeacherID
	if !req.User.HasPermission("course.manage") || teacherID == 0 {
		teacherID = req.User.ID
	}
	if req.Video != "" {
		return nil, xerror.InvalidParams.WithDetails("请通过课节附件添加已验证的课程资源")
	}
	course, err := s.buildCourse(req.GroupID, teacherID, req.Title, req.Intro, req.TeacherIntro, req.Cover, nil)
	if err != nil {
		return nil, err
	}
	course, err = s.Ds.CreateCourse(req.User, course)
	if err != nil {
		logrus.Errorf("Ds.CreateCourse err: %s", err)
		return nil, courseMutationError(err, web.ErrCreateCourseFailed)
	}
	return (*web.CreateCourseResp)(course.Format()), nil
}

func (s *courseAdminSrv) UpdateCourse(req *web.UpdateCourseReq) error {
	course, err := s.Ds.GetCourseByID(req.ID)
	if err != nil || course.Model == nil || course.ID <= 0 {
		return web.ErrCourseNotExist
	}
	teacherID := req.TeacherID
	if teacherID == 0 {
		teacherID = course.TeacherID
	}
	if !canManageCourse(req.User, course.TeacherID) ||
		(!req.User.HasPermission("course.manage") && teacherID != req.User.ID) {
		return web.ErrNoPermission
	}
	if req.Video != "" {
		return xerror.InvalidParams.WithDetails("请通过课节附件添加已验证的课程资源")
	}
	updated, err := s.buildCourse(req.GroupID, teacherID, req.Title, req.Intro, req.TeacherIntro, req.Cover, course)
	if err != nil {
		return err
	}
	updated.Model = course.Model
	if err := s.Ds.UpdateCourse(req.User, updated); err != nil {
		logrus.Errorf("Ds.UpdateCourse err: %s", err)
		return courseMutationError(err, web.ErrUpdateCourseFailed)
	}
	return nil
}

// buildCourse only edits catalog metadata. Resources belong to verified lessons.
func (s *courseAdminSrv) buildCourse(groupID, teacherID int64, title, intro, teacherIntro, cover string, exist *ms.Course) (*ms.Course, error) {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > 128 {
		return nil, xerror.InvalidParams.WithDetails("课程标题为1~128字")
	}
	if utf8.RuneCountInString(intro) > 2000 || utf8.RuneCountInString(teacherIntro) > 2000 {
		return nil, xerror.InvalidParams.WithDetails("课程及老师简介最长2000字")
	}
	if _, err := s.Ds.GetCourseGroupByID(groupID); err != nil {
		return nil, web.ErrCourseGroupNotExist
	}
	if _, err := s.Ds.GetUserByID(teacherID); err != nil {
		return nil, web.ErrCourseTeacherInvalid
	}
	cover = strings.TrimSpace(cover)
	if cover != "" {
		key := s.oss.ObjectKey(cover)
		if (!strings.HasPrefix(key, "public/image/") && !strings.HasPrefix(key, "image/")) || path.Clean(key) != key || strings.ContainsAny(key, "%?#\\") {
			return nil, xerror.InvalidParams.WithDetails("课程封面必须使用本站公开图片")
		}
		if err := s.oss.PersistObject(key); err != nil {
			return nil, web.ErrFileUploadFailed
		}
		if exists, err := s.oss.IsObjectExist(key); err != nil || !exists {
			return nil, web.ErrFileUploadFailed
		}
		cover = s.oss.ObjectURL(key)
	}
	course := &ms.Course{
		GroupID:      groupID,
		TeacherID:    teacherID,
		Title:        title,
		Intro:        intro,
		TeacherIntro: teacherIntro,
		Cover:        cover,
	}
	if exist != nil {
		course.VideoURL = exist.VideoURL
	}
	return course, nil
}

func (s *courseAdminSrv) DeleteCourse(req *web.DeleteCourseReq) error {
	course, err := s.Ds.GetCourseByID(req.ID)
	if err != nil || course.Model == nil || course.ID <= 0 {
		return web.ErrCourseNotExist
	}
	if !canManageCourse(req.User, course.TeacherID) {
		return web.ErrNoPermission
	}
	if err := s.Ds.DeleteCourse(req.User, course); err != nil {
		logrus.Errorf("Ds.DeleteCourse err: %s", err)
		return courseMutationError(err, web.ErrDeleteCourseFailed)
	}
	return nil
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
