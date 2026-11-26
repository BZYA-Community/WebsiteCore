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

func TestContentListsFilterByParentID(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{
		TablePrefix:   "ut_content_parent_" + hex.EncodeToString(suffix[:]) + "_",
		SingularTable: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	models := []any{&PostContent{}, &CommentContent{}, &CommentReply{}}
	t.Cleanup(func() {
		if err := db.Migrator().DropTable(models...); err != nil {
			t.Errorf("drop test tables: %v", err)
		}
	})
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatal(err)
	}

	const (
		parentID      int64 = 4
		otherParentID int64 = 9
		matchingRowID int64 = 5
	)
	// In each table, the unrelated row's primary key equals the requested parent ID.
	fixtures := []any{
		&[]PostContent{
			{Model: &Model{ID: parentID}, PostID: otherParentID},
			{Model: &Model{ID: matchingRowID}, PostID: parentID},
		},
		&[]CommentContent{
			{Model: &Model{ID: parentID}, CommentID: otherParentID},
			{Model: &Model{ID: matchingRowID}, CommentID: parentID},
		},
		&[]CommentReply{
			{Model: &Model{ID: parentID}, CommentID: otherParentID},
			{Model: &Model{ID: matchingRowID}, CommentID: parentID},
		},
	}
	for _, rows := range fixtures {
		if err := db.Create(rows).Error; err != nil {
			t.Fatalf("create %T fixtures: %v", rows, err)
		}
	}

	t.Run("PostContent", func(t *testing.T) {
		rows, err := (&PostContent{PostID: parentID}).List(db, &ConditionsT{}, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("List() returned %d rows, want 1", len(rows))
		}
		if rows[0].ID != matchingRowID {
			t.Errorf("List() returned row %d, want %d", rows[0].ID, matchingRowID)
		}
	})

	t.Run("CommentContent", func(t *testing.T) {
		rows, err := (&CommentContent{CommentID: parentID}).List(db, &ConditionsT{}, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("List() returned %d rows, want 1", len(rows))
		}
		if rows[0].ID != matchingRowID {
			t.Errorf("List() returned row %d, want %d", rows[0].ID, matchingRowID)
		}
	})

	t.Run("CommentReply", func(t *testing.T) {
		rows, err := (&CommentReply{CommentID: parentID}).List(db, &ConditionsT{}, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 {
			t.Fatalf("List() returned %d rows, want 1", len(rows))
		}
		if rows[0].ID != matchingRowID {
			t.Errorf("List() returned row %d, want %d", rows[0].ID, matchingRowID)
		}
	})
}
