package dbr

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestPostCollectionListUsesCurrentVisibility(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{
		TablePrefix: "ut_collection_" + hex.EncodeToString(buf) + "_", SingularTable: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Post{}, &PostCollection{}, &Following{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Migrator().DropTable(&PostCollection{}, &Following{}, &Post{}) })

	const viewer int64 = 10
	posts := []*Post{
		{Model: &Model{}, UserID: 20, Visibility: PostVisitPublic, AuditStatus: PostAuditApproved},
		{Model: &Model{}, UserID: 20, Visibility: PostVisitPublic, AuditStatus: PostAuditPending},
		{Model: &Model{}, UserID: 30, Visibility: PostVisitFollowing, AuditStatus: PostAuditApproved},
		{Model: &Model{}, UserID: 40, Visibility: PostVisitFollowing, AuditStatus: PostAuditApproved},
		{Model: &Model{}, UserID: viewer, Visibility: PostVisitPrivate, AuditStatus: PostAuditPending},
	}
	for _, post := range posts {
		if err := db.Create(post).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&PostCollection{Model: &Model{}, PostID: post.ID, UserID: viewer}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&Following{Model: &Model{}, UserId: viewer, FollowId: 30}).Error; err != nil {
		t.Fatal(err)
	}

	collection := &PostCollection{UserID: viewer}
	got, err := collection.List(db, &ConditionsT{}, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	count, err := collection.Count(db, &ConditionsT{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || count != 3 {
		t.Fatalf("list=%d count=%d, want 3 visible collections", len(got), count)
	}
}
