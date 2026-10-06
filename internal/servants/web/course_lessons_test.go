package web

import (
	"errors"
	"strings"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	model "github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
)

type courseCoverStorage struct {
	permissionStorage
	persisted string
	failure   error
}

func (s *courseCoverStorage) ObjectKey(value string) string {
	return strings.TrimPrefix(value, "https://storage.example.test/")
}

func (s *courseCoverStorage) ObjectURL(key string) string {
	return "https://storage.example.test/" + key
}

func (s *courseCoverStorage) PersistObject(key string) error {
	s.persisted = key
	return s.failure
}

func TestCourseCoverPublicationPrecedesCatalogWrite(t *testing.T) {
	actor := permissionUser(1, authz.CourseManageOwn)
	for _, tc := range []struct {
		name, cover, wantKey string
		storageError         error
		wantError            bool
	}{
		{name: "new public image is persisted", cover: "https://storage.example.test/public/image/cover.png", wantKey: "public/image/cover.png"},
		{name: "legacy public image is persisted", cover: "image/cover.png", wantKey: "image/cover.png"},
		{name: "storage failure blocks catalog", cover: "public/image/cover.png", wantKey: "public/image/cover.png", storageError: errors.New("publish failed"), wantError: true},
		{name: "private key never published", cover: "attachment/course/staging/resource.pdf", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ds := &permissionData{user: actor}
			storage := &courseCoverStorage{failure: tc.storageError}
			s := &courseAdminSrv{DaoServant: &base.DaoServant{Ds: ds}, oss: storage}
			_, err := s.CreateCourse(&model.CreateCourseReq{BaseInfo: model.BaseInfo{User: actor}, GroupID: 6, Title: "Course", Cover: tc.cover})
			if (err != nil) != tc.wantError || storage.persisted != tc.wantKey {
				t.Fatalf("err=%v persisted=%q", err, storage.persisted)
			}
			if tc.wantError && ds.course != nil {
				t.Fatal("failed cover publication still wrote catalog metadata")
			}
			if !tc.wantError && (ds.course == nil || ds.course.Cover != storage.ObjectURL(tc.wantKey)) {
				t.Fatalf("wrong public cover URL: %+v", ds.course)
			}
		})
	}
}

type courseLessonData struct {
	core.DataService
	lesson *ms.CourseLesson
	actor  *ms.User
}

func (d *courseLessonData) SaveCourseLesson(actor *ms.User, lesson *ms.CourseLesson) (*ms.CourseLesson, error) {
	d.actor, d.lesson = actor, lesson
	return lesson, nil
}

func TestCourseResourcesRequireCurrentViewPermission(t *testing.T) {
	s := &courseLooseSrv{}
	for _, user := range []*ms.User{nil, permissionUser(1, "course.catalog"), permissionUser(1, "course.manage_own")} {
		if _, err := s.CourseLessons(&model.CourseLessonsReq{BaseInfo: model.BaseInfo{User: user}, CourseID: 1}); err != model.ErrNoPermission {
			t.Fatalf("lessons without course.view: %v", err)
		}
		if _, err := s.CourseAttachment(&model.CourseAttachmentReq{BaseInfo: model.BaseInfo{User: user}, ID: 1}); err != model.ErrNoPermission {
			t.Fatalf("attachment without course.view: %v", err)
		}
		if _, err := s.CourseVideo(&model.CourseVideoReq{BaseInfo: model.BaseInfo{User: user}, ID: 1}); err != model.ErrNoPermission {
			t.Fatalf("legacy video without course.view: %v", err)
		}
	}
}

func TestLessonUpdatePreservesOmittedAttachmentArray(t *testing.T) {
	data := &courseLessonData{}
	s := &courseAdminSrv{DaoServant: &base.DaoServant{Ds: data}}
	actor := permissionUser(1, authz.CourseManageOwn)
	req := &model.UpdateCourseLessonReq{ID: 7, CourseLessonReq: model.CourseLessonReq{BaseInfo: model.BaseInfo{User: actor}, CourseID: 3, Title: "Lesson"}}
	if _, err := s.UpdateCourseLesson(req); err != nil || data.actor != actor || data.lesson.ID != 7 || data.lesson.Attachments != nil {
		t.Fatalf("omitted attachments changed: %+v,%v", data.lesson, err)
	}
	req.Attachments = []model.CourseLessonAttachmentInput{}
	if _, err := s.UpdateCourseLesson(req); err != nil || data.lesson.Attachments == nil || len(data.lesson.Attachments) != 0 {
		t.Fatalf("explicit clear lost: %+v,%v", data.lesson, err)
	}
}
