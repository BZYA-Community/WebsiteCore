package web

import (
	"errors"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	model "github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
)

type courseCoverStorage struct {
	sharedAttachmentStorage
	failure, checkError error
	missing             bool
}

func (s *courseCoverStorage) ObjectURL(key string) string {
	return "https://storage.example.test/" + key
}

func (s *courseCoverStorage) PersistObject(key string) error {
	s.persisted = append(s.persisted, key)
	return s.failure
}

func (s *courseCoverStorage) IsObjectExist(string) (bool, error) {
	return !s.missing, s.checkError
}

type courseCoverData struct {
	permissionData
	saveError error
	writes    int
}

func (d *courseCoverData) GetCourseByID(int64) (*ms.Course, error) { return d.course, nil }
func (d *courseCoverData) CreateCourse(actor *ms.User, course *ms.Course) (*ms.Course, error) {
	d.writes++
	if d.saveError != nil {
		return nil, d.saveError
	}
	return d.permissionData.CreateCourse(actor, course)
}
func (d *courseCoverData) UpdateCourse(_ *ms.User, course *ms.Course) error {
	d.writes++
	if d.saveError != nil {
		return d.saveError
	}
	d.course = course
	return nil
}

func TestCourseCoverPublicationPrecedesCatalogWrite(t *testing.T) {
	actor := permissionUser(1, authz.CourseManageOwn)
	const oldCover = "https://storage.example.test/public/image/old.jpg"
	for _, tc := range []struct {
		name, cover, wantKey string
		storageError         error
		checkError           error
		missing              bool
		saveError            error
		update               bool
		wantError            bool
	}{
		{name: "new public image is persisted", cover: "https://storage.example.test/public/image/cover.png", wantKey: "public/image/cover.png"},
		{name: "legacy public image is persisted", cover: "image/cover.png", wantKey: "image/cover.png"},
		{name: "course without cover can be created"},
		{name: "replacement preserves old shared cover", cover: "public/image/new.webp", wantKey: "public/image/new.webp", update: true},
		{name: "existing cover can be retained", cover: oldCover, wantKey: "public/image/old.jpg", update: true},
		{name: "storage failure blocks catalog", cover: "public/image/cover.png", wantKey: "public/image/cover.png", storageError: errors.New("publish failed"), wantError: true},
		{name: "replacement storage failure preserves catalog", cover: "public/image/new.webp", wantKey: "public/image/new.webp", storageError: errors.New("publish failed"), update: true, wantError: true},
		{name: "missing replacement blocks catalog", cover: "public/image/missing.webp", wantKey: "public/image/missing.webp", missing: true, update: true, wantError: true},
		{name: "replacement inspection failure blocks catalog", cover: "public/image/new.webp", wantKey: "public/image/new.webp", checkError: errors.New("inspect failed"), update: true, wantError: true},
		{name: "create database failure preserves shared cover", cover: oldCover, wantKey: "public/image/old.jpg", saveError: errors.New("insert failed"), wantError: true},
		{name: "update database failure preserves shared covers", cover: "public/image/new.webp", wantKey: "public/image/new.webp", saveError: errors.New("update failed"), update: true, wantError: true},
		{name: "private key never published", cover: "attachment/course/staging/resource.pdf", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ds := &courseCoverData{permissionData: permissionData{user: actor}, saveError: tc.saveError}
			if tc.update {
				ds.course = &ms.Course{Model: &ms.Model{ID: 3}, GroupID: 6, TeacherID: actor.ID, Title: "Original", Cover: oldCover}
			}
			original := ds.course
			storage := &courseCoverStorage{failure: tc.storageError, checkError: tc.checkError, missing: tc.missing}
			s := &courseAdminSrv{DaoServant: &base.DaoServant{Ds: ds}, oss: storage}
			var err error
			if tc.update {
				err = s.UpdateCourse(&model.UpdateCourseReq{BaseInfo: model.BaseInfo{User: actor}, ID: 3, GroupID: 6, Title: "Course", Cover: tc.cover})
			} else {
				_, err = s.CreateCourse(&model.CreateCourseReq{BaseInfo: model.BaseInfo{User: actor}, GroupID: 6, Title: "Course", Cover: tc.cover})
			}
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected error: %v", err)
			}
			if (tc.wantKey == "" && len(storage.persisted) != 0) || (tc.wantKey != "" && (len(storage.persisted) != 1 || storage.persisted[0] != tc.wantKey)) {
				t.Fatalf("err=%v persisted=%q", err, storage.persisted)
			}
			wantWrites := 1
			if tc.wantError && tc.saveError == nil {
				wantWrites = 0
			}
			if ds.writes != wantWrites || (tc.wantError && ds.course != original) {
				t.Fatalf("failed operation changed catalog: writes=%d course=%+v", ds.writes, ds.course)
			}
			if original != nil && (original.Cover != oldCover || original.Title != "Original") {
				t.Fatal("mutated the original course before saving")
			}
			wantCover := ""
			if tc.wantKey != "" {
				wantCover = storage.ObjectURL(tc.wantKey)
			}
			if !tc.wantError && (ds.course == nil || ds.course.Cover != wantCover) {
				t.Fatalf("wrong public cover URL: %+v", ds.course)
			}
			if len(storage.deleted) != 0 {
				t.Fatalf("deleted a shared cover: %v", storage.deleted)
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
