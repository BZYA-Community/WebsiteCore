package jinzhu

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestCountPublicTagRefsCountsEachPostOnce(t *testing.T) {
	got := countPublicTagRefs([]string{
		"approved, shared, shared",
		"shared",
		" , approved, ",
		"",
	})

	want := map[string]int64{
		"approved": 2,
		"shared":   2,
	}
	if len(got) != len(want) {
		t.Fatalf("tag count = %d, want %d: %#v", len(got), len(want), got)
	}
	for tag, count := range want {
		if got[tag] != count {
			t.Errorf("count[%q] = %d, want %d", tag, got[tag], count)
		}
	}
}

func TestListPublicPostTagFieldsExcludesHiddenPosts(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set, skip topic persistence test")
	}

	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}
	tablePrefix := "ut_topic_" + hex.EncodeToString(buf) + "_"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: tablePrefix, SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&dbr.Post{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Migrator().DropTable(&dbr.Post{}); err != nil {
			t.Logf("DropTable() error = %v", err)
		}
	})

	posts := []*dbr.Post{
		{Model: &dbr.Model{}, Tags: "approved", AuditStatus: dbr.PostAuditApproved, Visibility: dbr.PostVisitPublic},
		{Model: &dbr.Model{}, Tags: "pending", AuditStatus: dbr.PostAuditPending, Visibility: dbr.PostVisitPublic},
		{Model: &dbr.Model{}, Tags: "rejected", AuditStatus: dbr.PostAuditRejected, Visibility: dbr.PostVisitPublic},
		{Model: &dbr.Model{}, Tags: "private", AuditStatus: dbr.PostAuditApproved, Visibility: dbr.PostVisitPrivate},
		{Model: &dbr.Model{}, Tags: "deleted", AuditStatus: dbr.PostAuditApproved, Visibility: dbr.PostVisitPublic},
	}
	for _, post := range posts {
		if err := db.Create(post).Error; err != nil {
			t.Fatalf("create post %q: %v", post.Tags, err)
		}
	}
	if err := db.Model(posts[4]).Update("is_del", 1).Error; err != nil {
		t.Fatalf("mark deleted post: %v", err)
	}

	got, err := listPublicPostTagFields(db)
	if err != nil {
		t.Fatalf("listPublicPostTagFields() error = %v", err)
	}
	if len(got) != 1 || got[0] != "approved" {
		t.Fatalf("public tag fields = %#v, want [approved]", got)
	}
}
