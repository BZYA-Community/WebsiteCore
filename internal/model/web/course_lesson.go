package web

import "github.com/BZYA-Community/WebsiteCore/internal/core/ms"

type CourseLessonsReq struct {
	BaseInfo `form:"-" binding:"-"`
	CourseID int64 `form:"course_id" binding:"required"`
}

type CourseLessonsResp struct {
	Lessons []*ms.CourseLesson `json:"lessons"`
}

type CourseAttachmentReq struct {
	BaseInfo `form:"-" binding:"-"`
	ID       int64 `form:"id" binding:"required"`
}

type CourseAttachmentResp struct {
	SignedURL string `json:"signed_url"`
}

type CourseLessonAttachmentInput struct {
	AttachmentID int64  `json:"attachment_id" binding:"required"`
	Name         string `json:"name" binding:"required"`
	Kind         string `json:"kind" binding:"required"`
}

type CourseLessonReq struct {
	BaseInfo    `json:"-" binding:"-"`
	CourseID    int64                         `json:"course_id" binding:"required"`
	Title       string                        `json:"title" binding:"required"`
	Intro       string                        `json:"intro"`
	Sort        int                           `json:"sort"`
	Attachments []CourseLessonAttachmentInput `json:"attachments"`
}

type UpdateCourseLessonReq struct {
	CourseLessonReq
	ID int64 `json:"id" binding:"required"`
}

type DeleteCourseLessonReq struct {
	BaseInfo `json:"-" binding:"-"`
	ID       int64 `json:"id" binding:"required"`
}

type CourseLessonResp = ms.CourseLesson
