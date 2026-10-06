package jinzhu

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Storage's disk/SDK tests cover atomic promotion. This double injects failures
// at the database boundary without transferring gigabytes or using cloud keys.
type uploadTestStorage struct {
	core.ObjectStorageService
	objects map[string][]byte
}

func (s *uploadTestStorage) ObjectURL(key string) string { return "https://storage.test/" + key }
func (s *uploadTestStorage) ObjectKey(url string) string {
	return strings.TrimPrefix(url, "https://storage.test/")
}
func (s *uploadTestStorage) PutObject(key string, reader io.Reader, size int64, _ string, _ bool) (string, error) {
	if _, exists := s.objects[key]; exists {
		return "", fmt.Errorf("already exists")
	}
	data, err := io.ReadAll(io.LimitReader(reader, size+1))
	if err != nil || int64(len(data)) != size {
		return "", io.ErrUnexpectedEOF
	}
	s.objects[key] = data
	return s.ObjectURL(key), nil
}
func (s *uploadTestStorage) InspectObject(key string) (*core.ObjectMetadata, error) {
	data, ok := s.objects[key]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &core.ObjectMetadata{Header: data, Size: int64(len(data)), ContentType: "application/octet-stream", ETag: hex.EncodeToString(data)}, nil
}
func (s *uploadTestStorage) PromoteObject(source, destination, etag string) error {
	data, ok := s.objects[source]
	if !ok || hex.EncodeToString(data) != etag {
		return fmt.Errorf("source changed")
	}
	if existing, ok := s.objects[destination]; ok && !bytes.Equal(existing, data) {
		return fmt.Errorf("destination exists")
	}
	s.objects[destination] = bytes.Clone(data)
	return nil
}
func (s *uploadTestStorage) IsObjectExist(key string) (bool, error) {
	_, ok := s.objects[key]
	return ok, nil
}
func (s *uploadTestStorage) DeleteObject(key string) error { delete(s.objects, key); return nil }

func TestCourseUploadTransactions(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	var random [6]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	prefix := "ut_upload_" + hex.EncodeToString(random[:]) + "_"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix, SingularTable: true}})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	models := []any{&dbr.User{}, &dbr.IdentityGroup{}, &dbr.IdentityGroupPermission{}, &dbr.UserIdentityGroup{}, &dbr.Attachment{}, &dbr.OperationLog{}}
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
	guest := &dbr.IdentityGroup{Key: "guest", Name: "Guest"}
	group := &dbr.IdentityGroup{Key: "teacher", Name: "Teacher"}
	u := &ms.User{Model: &ms.Model{}, Status: ms.UserStatusNormal}
	other := &ms.User{Model: &ms.Model{}, Status: ms.UserStatusNormal, IsOperator: true}
	for _, row := range []any{guest, group, u, other} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{authz.CourseUpload, authz.CourseManageOwn} {
		if err := db.Create(&dbr.IdentityGroupPermission{GroupID: group.ID, Permission: p}).Error; err != nil {
			t.Fatal(err)
		}
	}
	grant := &dbr.UserIdentityGroup{UserID: u.ID, GroupID: group.ID}
	if err := db.Create(grant).Error; err != nil {
		t.Fatal(err)
	}
	s := &courseUploadSrv{db: db}
	oss := &uploadTestStorage{objects: map[string][]byte{}}
	payload := []byte("%PDF-1.7\ncourse material")
	start := func() *ms.Attachment {
		t.Helper()
		a, err := s.StartCourseUpload(u, "lesson.pdf", "resource", "application/pdf", int64(len(payload)))
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	for _, tc := range []struct {
		kind    string
		size    int64
		allowed bool
	}{
		{"resource", 2 << 30, true}, {"resource", (2 << 30) + 1, false}, {"attachment", 50 << 20, true}, {"attachment", (50 << 20) + 1, false}, {"resource", 0, false}, {"invalid", 1, false},
	} {
		_, err := s.StartCourseUpload(u, "lesson.pdf", tc.kind, "application/pdf", tc.size)
		if (err == nil) != tc.allowed {
			t.Fatalf("size boundary %+v: %v", tc, err)
		}
	}
	a := start()
	if err := s.WriteCourseUpload(a.ID, "image/png", bytes.NewReader(payload), oss); err == nil {
		t.Fatal("wrong MIME accepted")
	}
	if err := s.WriteCourseUpload(a.ID, "application/pdf", strings.NewReader("<html>malicious</html>"), oss); err == nil {
		t.Fatal("wrong signature accepted")
	}
	if err := s.WriteCourseUpload(a.ID, "application/pdf", bytes.NewReader(payload[:len(payload)-1]), oss); err == nil {
		t.Fatal("short body accepted")
	}
	if err := s.WriteCourseUpload(a.ID, "application/pdf", bytes.NewReader(append(bytes.Clone(payload), 1)), oss); err == nil {
		t.Fatal("long body accepted")
	}
	if err := s.WriteCourseUpload(a.ID, "application/pdf", bytes.NewReader(payload), oss); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteCourseUpload(a.ID, "application/pdf", bytes.NewReader(payload), oss); err == nil {
		t.Fatal("overwrite accepted")
	}
	if _, err := s.CompleteCourseUpload(other, a.ID, oss); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("wrong owner accepted: %v", err)
	}
	if err := db.Delete(grant).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompleteCourseUpload(u, a.ID, oss); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("revoked uploader accepted: %v", err)
	}
	if err := db.Create(grant).Error; err != nil {
		t.Fatal(err)
	}
	const callback = "test:upload_log_failure"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if log, ok := tx.Statement.Dest.(*dbr.OperationLog); ok && log.Action == "upload.complete" {
			tx.AddError(errors.New("injected operation log failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompleteCourseUpload(u, a.ID, oss); err == nil {
		t.Fatal("ignored log failure")
	}
	if err := db.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	var saved ms.Attachment
	if err := db.First(&saved, a.ID).Error; err != nil || saved.Verified || saved.Content != a.Content {
		t.Fatalf("failed transaction published attachment: %v", err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := s.CompleteCourseUpload(u, a.ID, oss)
			if err != nil || !got.Verified {
				t.Errorf("completion retry failed: %v", err)
			}
		}()
	}
	wg.Wait()
	var logs int64
	if err := db.Model(&dbr.OperationLog{}).Where("action = ? AND entity_id = ?", "upload.complete", a.ID).Count(&logs).Error; err != nil || logs != 1 {
		t.Fatalf("completion logs=%d: %v", logs, err)
	}
	if exists, _ := oss.IsObjectExist(a.Content); exists {
		t.Fatal("staging object retained after commit")
	}
	if exists, _ := oss.IsObjectExist(finalCourseKey(a.Content)); !exists {
		t.Fatal("verified object missing")
	}
	expired := start()
	if err := s.WriteCourseUpload(expired.ID, "application/pdf", bytes.NewReader(payload), oss); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ms.Attachment{}).Where("id IN ?", []int64{a.ID, expired.ID}).Update("upload_expires_on", time.Now().Unix()-86401).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompleteCourseUpload(u, expired.ID, oss); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("expired intent accepted: %v", err)
	}
	if err := s.ExpireCourseUploads(oss); err != nil {
		t.Fatal(err)
	}
	if exists, _ := oss.IsObjectExist(expired.Content); exists {
		t.Fatal("abandoned stage retained")
	}
	if exists, _ := oss.IsObjectExist(finalCourseKey(a.Content)); !exists {
		t.Fatal("cleanup deleted verified course resource")
	}
	saved = ms.Attachment{}
	if err := db.First(&saved, expired.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expired intent retained: %v", err)
	}
}
