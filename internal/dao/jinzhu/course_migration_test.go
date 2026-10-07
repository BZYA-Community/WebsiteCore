package jinzhu

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestCommunityMigrationsRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	var random [6]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	schema := "ut_framework_migration_" + hex.EncodeToString(random[:])
	if err := tx.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec("SET LOCAL search_path TO " + schema).Error; err != nil {
		t.Fatal(err)
	}
	const directory = "../../../scripts/migration/postgres/"
	run := func(path string) {
		t.Helper()
		script, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Exec(string(script)).Error; err != nil {
			t.Fatalf("%s: %v", filepath.Base(path), err)
		}
	}
	paths, err := filepath.Glob(directory + "*.up.sql")
	if err != nil || len(paths) == 0 {
		t.Fatal("migration files missing", err)
	}
	for _, path := range paths {
		run(path)
	}
	if err := tx.Exec("INSERT INTO p_user(username) VALUES ('migration-preserved-user')").Error; err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"0032_course_uploads", "0031_contact_verification", "0030_review_assignment", "0026_course_lessons"} {
		run(directory + name + ".down.sql")
	}
	for _, name := range []string{"0026_course_lessons", "0030_review_assignment", "0031_contact_verification", "0032_course_uploads"} {
		run(directory + name + ".up.sql")
	}
	var count int64
	if err := tx.Raw("SELECT count(*) FROM p_user WHERE username='migration-preserved-user'").Scan(&count).Error; err != nil || count != 1 {
		t.Fatalf("migration removed existing user: %v", err)
	}
	for _, table := range []string{"p_course_lesson", "p_review_task", "p_contact_verification", "p_operation_log"} {
		if err := tx.Exec("SELECT 1 FROM " + table + " LIMIT 0").Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Exec("SELECT email FROM p_user LIMIT 0").Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec("SELECT purpose,verified,mime_type,name,upload_expires_on FROM p_attachment LIMIT 0").Error; err != nil {
		t.Fatal(err)
	}
}

func TestCourseMigrationPreservesLegacyRecords(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; PostgreSQL course migration integration test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	// A transaction-local schema exercises the actual checked-in migration SQL
	// while preserving the development database and all unrelated fixtures.
	schemaName := "ut_course_migration_" + hex.EncodeToString(buf)
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	if err := tx.Exec("CREATE SCHEMA " + schemaName).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec("SET LOCAL search_path TO " + schemaName).Error; err != nil {
		t.Fatal(err)
	}
	runMigration := func(name string) {
		t.Helper()
		sql, err := os.ReadFile("../../../scripts/migration/postgres/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Exec(string(sql)).Error; err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	runMigration("0022_course_module.up.sql")
	if err := tx.Exec(`CREATE TABLE p_attachment (id BIGSERIAL PRIMARY KEY, content TEXT NOT NULL);
INSERT INTO p_attachment(content) VALUES ('attachment/course/existing');
INSERT INTO p_course_group(name) VALUES ('Existing category');
INSERT INTO p_course(group_id,teacher_id,title,video_url) VALUES (1,1,'Existing course','attachment/course/existing');`).Error; err != nil {
		t.Fatal(err)
	}
	runMigration("0026_course_lessons.up.sql")
	if err := tx.Exec(`INSERT INTO p_course_lesson(course_id,title) VALUES (1,'Lesson');
INSERT INTO p_course_lesson_attachment(lesson_id,attachment_id,name,kind) VALUES (1,1,'File','resource');`).Error; err != nil {
		t.Fatal(err)
	}
	runMigration("0026_course_lessons.down.sql")
	runMigration("0026_course_lessons.up.sql")
	var course struct{ Title, VideoURL string }
	if err := tx.Raw("SELECT title,video_url FROM p_course WHERE id=1").Scan(&course).Error; err != nil || course.Title != "Existing course" || course.VideoURL != "attachment/course/existing" {
		t.Fatalf("migration changed original course: %+v, %v", course, err)
	}
	var attachments int64
	if err := tx.Raw("SELECT COUNT(*) FROM p_attachment").Scan(&attachments).Error; err != nil || attachments != 1 {
		t.Fatalf("migration deleted original uploads: %d, %v", attachments, err)
	}
}
