package jinzhu

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestCourseAttachmentLimits(t *testing.T) {
	for _, tc := range []struct {
		kind, purpose  string
		size           int64
		verified, want bool
	}{
		{"attachment", "course_attachment", 50 << 20, true, true},
		{"attachment", "course_attachment", (50 << 20) + 1, true, false},
		{"resource", "course_resource", 2 << 30, true, true},
		{"resource", "course_resource", (2 << 30) + 1, true, false},
		{"resource", "course_resource", 10, false, false},
		{"attachment", "course_resource", 10, true, false},
		{"resource", "course_resource", 0, true, false},
		{"unknown", "course_unknown", 10, true, false},
	} {
		upload := &ms.Attachment{Verified: tc.verified, Purpose: tc.purpose, FileSize: tc.size, Content: "private-key", MimeType: "application/pdf"}
		if got := courseAttachmentValid(upload, tc.kind); got != tc.want {
			t.Errorf("%+v: got %v", tc, got)
		}
	}
}

func TestCourseCatalogDoesNotExposeResources(t *testing.T) {
	course := &ms.Course{Model: &ms.Model{ID: 1}, VideoURL: "private-resource-key", TeacherIntro: "A teacher"}
	encoded, err := json.Marshal(course.Format())
	if err != nil || strings.Contains(string(encoded), "private-resource-key") || strings.Contains(string(encoded), "video_url") {
		t.Fatalf("catalog leaked a resource: %s (%v)", encoded, err)
	}
}

func TestCourseStructureTransactions(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; PostgreSQL course integration test")
	}
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	prefix := "ut_course_" + hex.EncodeToString(buf) + "_"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix, SingularTable: true}})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	models := []any{&dbr.User{}, &dbr.IdentityGroup{}, &dbr.IdentityGroupPermission{}, &dbr.UserIdentityGroup{}, &dbr.Attachment{},
		&dbr.CourseGroup{}, &dbr.Course{}, &dbr.CourseLesson{}, &dbr.CourseLessonAttachment{}, &dbr.CourseComment{},
		&dbr.CourseCommentContent{}, &dbr.CourseCommentReply{}, &dbr.OperationLog{}, &dbr.ReviewTask{}, &dbr.ReviewTaskEvent{}}
	t.Cleanup(func() {
		for i := len(models) - 1; i >= 0; i-- {
			if err := db.Migrator().DropTable(models[i]); err != nil {
				t.Error(err)
			}
		}
	})
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&dbr.IdentityGroup{Key: "guest", Name: "Guest", Builtin: true}).Error; err != nil {
		t.Fatal(err)
	}
	createUser := func(operator bool, permissions ...string) *ms.User {
		t.Helper()
		user := &ms.User{Model: &ms.Model{}, Status: ms.UserStatusNormal, IsOperator: operator}
		if err := db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
		group := &dbr.IdentityGroup{Key: fmt.Sprintf("user%d", user.ID), Name: "Test grant"}
		if err := db.Create(group).Error; err != nil {
			t.Fatal(err)
		}
		for _, p := range permissions {
			if err := db.Create(&dbr.IdentityGroupPermission{GroupID: group.ID, Permission: p}).Error; err != nil {
				t.Fatal(err)
			}
		}
		if !operator {
			if err := db.Create(&dbr.UserIdentityGroup{UserID: user.ID, GroupID: group.ID}).Error; err != nil {
				t.Fatal(err)
			}
		}
		return user
	}
	admin, teacher, other := createUser(true), createUser(false, authz.CourseManageOwn), createUser(false, authz.CourseManageOwn)
	s, read := &courseManageSrv{db: db}, &courseSrv{db: db}
	parent, err := s.CreateCourseGroup(admin, &ms.CourseGroup{Name: "Machine learning"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.CreateCourseGroup(admin, &ms.CourseGroup{Name: "Supervised", ParentID: parent.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateCourseGroup(teacher, &ms.CourseGroup{Name: "Forbidden"}); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("teacher created category: %v", err)
	}
	if err := s.UpdateCourseGroup(admin, &ms.CourseGroup{Model: parent.Model, Name: parent.Name, ParentID: child.ID}); !errors.Is(err, core.ErrCourseCategoryInvalid) {
		t.Fatalf("accepted category cycle: %v", err)
	}
	if err := s.DeleteCourseGroup(admin, parent.ID); !errors.Is(err, core.ErrCourseCategoryNotEmpty) {
		t.Fatalf("deleted parent category: %v", err)
	}
	if _, err := s.CreateCourseGroup(admin, &ms.CourseGroup{Name: "Orphan", ParentID: 999999}); !errors.Is(err, core.ErrCourseCategoryInvalid) {
		t.Fatalf("orphan category: %v", err)
	}
	// Opposite concurrent reparenting operations must not create a cycle.
	left, err := s.CreateCourseGroup(admin, &ms.CourseGroup{Name: "Left"})
	if err != nil {
		t.Fatal(err)
	}
	right, err := s.CreateCourseGroup(admin, &ms.CourseGroup{Name: "Right"})
	if err != nil {
		t.Fatal(err)
	}
	start, results := make(chan struct{}), make(chan error, 2)
	for _, move := range []*ms.CourseGroup{
		{Model: left.Model, Name: left.Name, ParentID: right.ID},
		{Model: right.Model, Name: right.Name, ParentID: left.ID},
	} {
		go func(group *ms.CourseGroup) {
			<-start
			results <- s.UpdateCourseGroup(admin, group)
		}(move)
	}
	close(start)
	accepted, rejected := 0, 0
	for i := 0; i < 2; i++ {
		switch err := <-results; {
		case err == nil:
			accepted++
		case errors.Is(err, core.ErrCourseCategoryInvalid):
			rejected++
		default:
			t.Fatalf("concurrent category update: %v", err)
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatalf("category race accepted=%d rejected=%d", accepted, rejected)
	}
	course, err := s.CreateCourse(teacher, &ms.Course{GroupID: child.ID, TeacherID: other.ID, Title: "KNN", TeacherIntro: "Teacher biography"})
	if err != nil || course.TeacherID != teacher.ID {
		t.Fatalf("course ownership: %+v, %v", course, err)
	}
	if _, count, err := read.ListCourses(parent.ID, "", 0, 20); err != nil || count != 1 {
		t.Fatalf("descendant catalog: %d, %v", count, err)
	}
	if err := s.DeleteCourseGroup(admin, child.ID); !errors.Is(err, core.ErrCourseCategoryNotEmpty) {
		t.Fatalf("deleted occupied category: %v", err)
	}
	createUpload := func(user *ms.User, verified bool, purpose string, size int64) *ms.Attachment {
		t.Helper()
		upload := &ms.Attachment{Model: &ms.Model{}, UserID: user.ID, Verified: verified, Purpose: purpose, FileSize: size, MimeType: "application/pdf", Content: "attachment/course/private-key"}
		if err := db.Create(upload).Error; err != nil {
			t.Fatal(err)
		}
		return upload
	}
	first, resource := createUpload(teacher, true, "course_attachment", 50<<20), createUpload(teacher, true, "course_resource", 2<<30)
	lesson, err := s.SaveCourseLesson(teacher, &ms.CourseLesson{CourseID: course.ID, Title: "Nearest neighbors", Attachments: []*ms.CourseLessonAttachment{
		{AttachmentID: first.ID, Name: "Notes.pdf", Kind: "attachment"}, {AttachmentID: resource.ID, Name: "Dataset", Kind: "resource"},
	}})
	if err != nil || len(lesson.Attachments) != 2 {
		t.Fatalf("multi-attachment lesson: %+v, %v", lesson, err)
	}
	stableReferenceID := lesson.Attachments[0].ID
	if upload, err := read.GetCourseLessonAttachment(stableReferenceID); err != nil || upload.ID != first.ID {
		t.Fatalf("resolve resource: %+v, %v", upload, err)
	}
	if _, err := s.SaveCourseLesson(other, &ms.CourseLesson{CourseID: course.ID, Title: "Intruder"}); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("cross-owner lesson: %v", err)
	}
	for _, invalid := range []*ms.Attachment{
		createUpload(other, true, "course_attachment", 100), createUpload(teacher, false, "course_attachment", 100),
		createUpload(teacher, true, "course_resource", 100), createUpload(teacher, true, "course_attachment", (50<<20)+1),
	} {
		if _, err := s.SaveCourseLesson(teacher, &ms.CourseLesson{CourseID: course.ID, Title: "Invalid", Attachments: []*ms.CourseLessonAttachment{{AttachmentID: invalid.ID, Name: "File", Kind: "attachment"}}}); !errors.Is(err, core.ErrCourseAttachmentInvalid) {
			t.Fatalf("unsafe upload accepted: %v", err)
		}
	}
	// Metadata edits preserve lessons and omitted attachment arrays preserve links.
	course.Title = "Updated course"
	if err := s.UpdateCourse(teacher, course); err != nil {
		t.Fatal(err)
	}
	updated, err := s.SaveCourseLesson(teacher, &ms.CourseLesson{Model: lesson.Model, CourseID: course.ID, Title: "Updated lesson"})
	if err != nil || len(updated.Attachments) != 2 || updated.Attachments[0].ID != stableReferenceID {
		t.Fatalf("lost stable references: %+v, %v", updated, err)
	}
	// A global manager may retain existing references, but cannot import another
	// author's file into a different lesson.
	if _, err := s.SaveCourseLesson(admin, &ms.CourseLesson{Model: lesson.Model, CourseID: course.ID, Title: "Managed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveCourseLesson(admin, &ms.CourseLesson{CourseID: course.ID, Title: "Stolen", Attachments: []*ms.CourseLessonAttachment{{AttachmentID: first.ID, Name: "File", Kind: "attachment"}}}); !errors.Is(err, core.ErrCourseAttachmentInvalid) {
		t.Fatalf("admin stole upload: %v", err)
	}
	shared, err := s.SaveCourseLesson(teacher, &ms.CourseLesson{CourseID: course.ID, Title: "Shared", Attachments: []*ms.CourseLessonAttachment{{AttachmentID: first.ID, Name: "Shared", Kind: "attachment"}}})
	if err != nil {
		t.Fatal(err)
	}
	// Metadata reads never return resource keys, even for authenticated viewers.
	visible, err := read.GetCourseLessons(course.ID)
	encoded, marshalErr := json.Marshal(visible)
	if err != nil || marshalErr != nil || strings.Contains(string(encoded), "private-key") {
		t.Fatalf("lesson metadata exposed resource keys: %s, %v, %v", encoded, err, marshalErr)
	}
	// Audit persistence failures must roll back the metadata and reference edits.
	if err := db.Exec("ALTER TABLE " + prefix + "operation_log ADD CONSTRAINT reject_update CHECK (action <> 'update') NOT VALID").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveCourseLesson(teacher, &ms.CourseLesson{Model: lesson.Model, CourseID: course.ID, Title: "Must roll back", Attachments: []*ms.CourseLessonAttachment{}}); err == nil {
		t.Fatal("ignored audit failure")
	}
	lessons, err := read.GetCourseLessons(course.ID)
	if err != nil || len(lessons) != 2 || lessons[0].Title != "Managed" || len(lessons[0].Attachments) != 2 {
		t.Fatalf("audit failure lost data: %+v, %v", lessons, err)
	}
	if err := db.Exec("ALTER TABLE " + prefix + "operation_log DROP CONSTRAINT reject_update").Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCourseLesson(other, lesson.ID); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("cross-owner deletion: %v", err)
	}
	if err := s.DeleteCourseLesson(teacher, shared.ID); err != nil {
		t.Fatal(err)
	}
	var uploadCount int64
	if err := db.Model(&dbr.Attachment{}).Where("id = ?", first.ID).Count(&uploadCount).Error; err != nil || uploadCount != 1 {
		t.Fatalf("shared upload deleted: %d, %v", uploadCount, err)
	}
	// Reassignment and revocation take effect without trusting stale actor data.
	course.TeacherID = other.ID
	if err := s.UpdateCourse(admin, course); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCourse(teacher, course); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("former teacher retained access: %v", err)
	}
	if _, err := s.SaveCourseLesson(other, &ms.CourseLesson{Model: lesson.Model, CourseID: course.ID, Title: "Transferred"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("user_id = ?", other.ID).Delete(&dbr.UserIdentityGroup{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCourseLesson(other, lesson.ID); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("revoked grant retained: %v", err)
	}
	if err := s.DeleteCourse(admin, course); err != nil {
		t.Fatal(err)
	}
	if _, err := read.GetCourseLessonAttachment(stableReferenceID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted course kept resource access: %v", err)
	}
	if err := db.Model(&dbr.Attachment{}).Where("id = ?", first.ID).Count(&uploadCount).Error; err != nil || uploadCount != 1 {
		t.Fatalf("course deletion removed upload: %d, %v", uploadCount, err)
	}
	if err := s.DeleteCourseGroup(admin, child.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCourseGroup(admin, parent.ID); err != nil {
		t.Fatal(err)
	}
}
