package web

import (
	"errors"
	"strings"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	model "github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
)

type courseStorageData struct {
	core.DataService
	course *ms.Course
	writes int
}

func (d *courseStorageData) GetCourseGroupByID(int64) (*ms.CourseGroup, error) {
	return &ms.CourseGroup{Model: &ms.Model{ID: 1}}, nil
}
func (d *courseStorageData) GetUserByID(int64) (*ms.User, error) {
	return &ms.User{Model: &ms.Model{ID: 2}}, nil
}
func (d *courseStorageData) GetCourseByID(int64) (*ms.Course, error) { return d.course, nil }
func (d *courseStorageData) CreateCourse(c *ms.Course) (*ms.Course, error) {
	c.Model = &ms.Model{ID: 3}
	d.course, d.writes = c, d.writes+1
	return c, nil
}
func (d *courseStorageData) UpdateCourse(c *ms.Course) error {
	d.course, d.writes = c, d.writes+1
	return nil
}

// Model the shared interface: TempDir uploads are invisible at the final key
// until PersistObject succeeds. No provider credentials or network are used.
type courseStorageObjects struct {
	core.ObjectStorageService
	temporary, final      map[string]bool
	persistErr, existsErr error
	persistCalls          int
	deleted               []string
}

func (s *courseStorageObjects) ObjectKey(url string) string {
	return strings.TrimPrefix(url, "https://storage.example/")
}
func (s *courseStorageObjects) ObjectURL(key string) string { return "https://storage.example/" + key }
func (s *courseStorageObjects) IsObjectExist(key string) (bool, error) {
	return s.final[key], s.existsErr
}
func (s *courseStorageObjects) PersistObject(key string) error {
	s.persistCalls++
	if s.persistErr != nil {
		return s.persistErr
	}
	if s.temporary[key] {
		s.final[key] = true
		delete(s.temporary, key)
	}
	return nil
}
func (s *courseStorageObjects) DeleteObjects(keys []string) error {
	s.deleted = append(s.deleted, keys...)
	for _, key := range keys {
		delete(s.final, key)
	}
	return nil
}

func TestCourseVideoPersistence(t *testing.T) {
	const oldKey = courseVideoPrefix + "old.mp4"
	const newKey = courseVideoPrefix + "new.mp4"
	for _, replace := range []bool{false, true} {
		for _, mode := range []string{"temporary", "final", "missing", "persist failure", "exists failure"} {
			name := "create/" + mode
			if replace {
				name = "replace/" + mode
			}
			t.Run(name, func(t *testing.T) {
				storage := &courseStorageObjects{temporary: map[string]bool{}, final: map[string]bool{oldKey: true}}
				switch mode {
				case "temporary":
					storage.temporary[newKey] = true
				case "final":
					storage.final[newKey] = true
				case "persist failure":
					storage.final[newKey] = true // errors must not be ignored even if a final key exists
					storage.persistErr = errors.New("copy failed")
				case "exists failure":
					storage.temporary[newKey] = true
					storage.existsErr = errors.New("head failed")
				}
				old := &ms.Course{Model: &ms.Model{ID: 3}, VideoURL: storage.ObjectURL(oldKey)}
				database := &courseStorageData{course: old}
				srv := &courseAdminSrv{DaoServant: &base.DaoServant{Ds: database}, oss: storage}
				var err error
				if replace {
					err = srv.UpdateCourse(&model.UpdateCourseReq{ID: 3, GroupID: 1, TeacherID: 2, Title: "course", Video: storage.ObjectURL(newKey)})
				} else {
					_, err = srv.CreateCourse(&model.CreateCourseReq{GroupID: 1, TeacherID: 2, Title: "course", Video: newKey})
				}
				ok := mode == "temporary" || mode == "final"
				if (err == nil) != ok {
					t.Fatalf("save: %v; want success=%v", err, ok)
				}
				if ok {
					if database.writes != 1 || database.course.VideoURL != storage.ObjectURL(newKey) || !storage.final[newKey] || storage.temporary[newKey] {
						t.Fatal("course not saved with a durable video")
					}
					if replace && (len(storage.deleted) != 1 || storage.deleted[0] != oldKey) {
						t.Fatal("old video not cleaned after replacement")
					}
				} else if database.writes != 0 || len(storage.deleted) != 0 || database.course != old || !storage.final[oldKey] {
					t.Fatal("failed save changed the course or deleted old video")
				}
			})
		}
	}
}

func TestCourseUnchangedVideoDoesNotPersistAgain(t *testing.T) {
	storage := &courseStorageObjects{persistErr: errors.New("must not persist")}
	database := &courseStorageData{course: &ms.Course{Model: &ms.Model{ID: 3}, VideoURL: storage.ObjectURL(courseVideoPrefix + "old.mp4")}}
	srv := &courseAdminSrv{DaoServant: &base.DaoServant{Ds: database}, oss: storage}
	if err := srv.UpdateCourse(&model.UpdateCourseReq{ID: 3, GroupID: 1, TeacherID: 2, Title: "edited"}); err != nil {
		t.Fatal(err)
	}
	if storage.persistCalls != 0 || database.writes != 1 {
		t.Fatal("unchanged video was persisted again")
	}
}
