package web

import (
	"errors"
	"strings"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func courseMutationError(err error, fallback *xerror.Error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, authz.ErrDenied):
		return web.ErrNoPermission
	case errors.Is(err, core.ErrCourseCategoryNotEmpty):
		return web.ErrCourseGroupNotEmpty
	case errors.Is(err, core.ErrCourseCategoryInvalid):
		return xerror.InvalidParams.WithDetails("课程分类的父级不存在或形成循环")
	case errors.Is(err, core.ErrCourseAttachmentInvalid):
		return xerror.InvalidParams.WithDetails("附件未通过验证、超出大小限制或不属于当前用户")
	case errors.Is(err, authz.ErrInvalid):
		return xerror.InvalidParams
	case errors.Is(err, gorm.ErrRecordNotFound):
		return web.ErrCourseNotExist
	default:
		logrus.Errorf("course operation failed: %s", err)
		return fallback
	}
}

func (s *courseLooseSrv) CourseLessons(req *web.CourseLessonsReq) (*web.CourseLessonsResp, error) {
	if !req.User.HasPermission(authz.CourseView) {
		return nil, web.ErrNoPermission
	}
	lessons, err := s.Ds.GetCourseLessons(req.CourseID)
	if err != nil {
		return nil, courseMutationError(err, web.ErrGetCourseListFailed)
	}
	return &web.CourseLessonsResp{Lessons: lessons}, nil
}

func (s *courseLooseSrv) CourseAttachment(req *web.CourseAttachmentReq) (*web.CourseAttachmentResp, error) {
	if !req.User.HasPermission(authz.CourseView) {
		return nil, web.ErrNoPermission
	}
	upload, err := s.Ds.GetCourseLessonAttachment(req.ID)
	if err != nil {
		return nil, courseMutationError(err, web.ErrCourseVideoInvalid)
	}
	key := s.oss.ObjectKey(upload.Content)
	if !strings.HasPrefix(key, courseVideoPrefix) {
		return nil, web.ErrCourseVideoInvalid
	}
	signedURL, err := s.oss.SignURL(key, courseVideoSignExpireSec)
	if err != nil {
		return nil, web.ErrCourseVideoInvalid
	}
	return &web.CourseAttachmentResp{SignedURL: signedURL}, nil
}

func courseLessonFrom(req *web.CourseLessonReq, id int64) *ms.CourseLesson {
	lesson := &ms.CourseLesson{Model: &ms.Model{ID: id}, CourseID: req.CourseID, Title: req.Title, Intro: req.Intro, Sort: req.Sort}
	if req.Attachments != nil {
		lesson.Attachments = make([]*ms.CourseLessonAttachment, 0, len(req.Attachments))
		for _, input := range req.Attachments {
			lesson.Attachments = append(lesson.Attachments, &ms.CourseLessonAttachment{
				AttachmentID: input.AttachmentID, Name: input.Name, Kind: input.Kind,
			})
		}
	}
	return lesson
}

func (s *courseAdminSrv) CreateCourseLesson(req *web.CourseLessonReq) (*web.CourseLessonResp, error) {
	lesson, err := s.Ds.SaveCourseLesson(req.User, courseLessonFrom(req, 0))
	return lesson, courseMutationError(err, web.ErrCreateCourseFailed)
}

func (s *courseAdminSrv) UpdateCourseLesson(req *web.UpdateCourseLessonReq) (*web.CourseLessonResp, error) {
	lesson, err := s.Ds.SaveCourseLesson(req.User, courseLessonFrom(&req.CourseLessonReq, req.ID))
	return lesson, courseMutationError(err, web.ErrUpdateCourseFailed)
}

func (s *courseAdminSrv) DeleteCourseLesson(req *web.DeleteCourseLessonReq) error {
	return courseMutationError(s.Ds.DeleteCourseLesson(req.User, req.ID), web.ErrDeleteCourseFailed)
}
