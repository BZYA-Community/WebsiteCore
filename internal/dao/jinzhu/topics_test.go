package jinzhu

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestUpsertTagsConcurrent(t *testing.T) {
	db := newTopicTestDB(t, &dbr.Tag{})
	tagTable := db.NamingStrategy.TableName("Tag")
	if err := db.Exec("CREATE UNIQUE INDEX " + tagTable + "_name ON " + tagTable + " (tag)").Error; err != nil {
		t.Fatal(err)
	}

	const originalUserID = 1
	for _, tc := range []struct {
		name         string
		initialCount int64
		deleted      bool
	}{
		{name: "new"},
		{name: "existing", initialCount: 5},
		{name: "soft-deleted", initialCount: 5, deleted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			db := db.WithContext(ctx)
			seed := &dbr.Tag{Model: &dbr.Model{}, UserID: originalUserID, Tag: tc.name, QuoteNum: tc.initialCount}
			if tc.deleted {
				seed.IsDel = 1
				seed.DeletedOn = time.Now().Unix()
			}
			if tc.initialCount > 0 {
				if err := db.Create(seed).Error; err != nil {
					t.Fatal(err)
				}
			}

			first := db.Begin()
			if first.Error != nil {
				t.Fatal(first.Error)
			}
			t.Cleanup(func() { first.Rollback() })
			firstTags, err := createTags(first, originalUserID, []string{tc.name})
			if err != nil {
				t.Fatal(err)
			}

			second := db.Begin()
			if second.Error != nil {
				t.Fatal(second.Error)
			}
			t.Cleanup(func() { second.Rollback() })
			var pid int
			if err := second.Raw("SELECT pg_backend_pid()").Scan(&pid).Error; err != nil {
				t.Fatal(err)
			}
			type result struct {
				tags cs.TagInfoList
				err  error
			}
			done := make(chan result, 1)
			go func() {
				tags, err := createTags(second, originalUserID+1, []string{tc.name})
				done <- result{tags: tags, err: err}
			}()

			// Hold the first write open until PostgreSQL reports the competing write waiting.
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()
			for {
				var blocked bool
				if err := db.Raw("SELECT cardinality(pg_blocking_pids(?)) > 0", pid).Scan(&blocked).Error; err != nil {
					t.Fatalf("wait for concurrent tag write: %v", err)
				}
				if blocked {
					break
				}
				select {
				case outcome := <-done:
					t.Fatalf("second write finished before first commit: %v", outcome.err)
				case <-ticker.C:
				}
			}
			if err := first.Commit().Error; err != nil {
				t.Fatal(err)
			}
			outcome := <-done
			if outcome.err != nil {
				t.Fatalf("second tag write: %v", outcome.err)
			}
			if err := second.Commit().Error; err != nil {
				t.Fatal(err)
			}

			var stored dbr.Tag
			if err := db.Unscoped().Where("tag = ?", tc.name).First(&stored).Error; err != nil {
				t.Fatal(err)
			}
			wantCount := tc.initialCount + 2
			if stored.QuoteNum != wantCount {
				t.Errorf("stored reference count = %d, want %d", stored.QuoteNum, wantCount)
			}
			if stored.UserID != originalUserID || (tc.initialCount > 0 && stored.ID != seed.ID) {
				t.Errorf("tag identity changed: ID %d, owner %d", stored.ID, stored.UserID)
			}
			if stored.IsDel != seed.IsDel || stored.DeletedOn != seed.DeletedOn {
				t.Errorf("stored is_del = %d, deleted_on = %d, want %d and %d", stored.IsDel, stored.DeletedOn, seed.IsDel, seed.DeletedOn)
			}
			for i, tags := range []cs.TagInfoList{firstTags, outcome.tags} {
				want := cs.TagInfo{ID: stored.ID, UserID: originalUserID, Tag: tc.name, QuoteNum: tc.initialCount + int64(i) + 1}
				if len(tags) != 1 || tags[0] == nil {
					t.Fatalf("write %d returned %v, want one tag", i+1, tags)
				}
				if got := *tags[0]; got != want {
					t.Errorf("write %d returned %+v, want %+v", i+1, got, want)
				}
			}
		})
	}
}

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
	db := newTopicTestDB(t, &dbr.Post{})

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

func newTopicTestDB(t *testing.T, model any) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; PostgreSQL topic integration test")
	}
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("generate topic test table prefix: %v", err)
	}
	prefix := "ut_topic_" + hex.EncodeToString(buf) + "_"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: prefix, SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open PostgreSQL test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	t.Cleanup(func() {
		if err := db.Migrator().DropTable(model); err != nil {
			t.Errorf("drop topic test table: %v", err)
		}
	})
	if err := db.AutoMigrate(model); err != nil {
		t.Fatalf("migrate topic test table: %v", err)
	}
	return db
}
