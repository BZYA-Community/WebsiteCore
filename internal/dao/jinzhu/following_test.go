package jinzhu

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"slices"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestUserFollowingUniqueness(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; PostgreSQL user follow integration test")
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var token [6]byte
	_, err := rand.Read(token[:])
	must(err)
	schemaName := "ut_user_follow_" + hex.EncodeToString(token[:])
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: schemaName + ".p_", SingularTable: true},
	})
	must(err)
	sqlDB, err := db.DB()
	must(err)
	defer sqlDB.Close()
	must(db.Exec("CREATE SCHEMA " + schemaName).Error)
	defer db.Exec("DROP SCHEMA " + schemaName + " CASCADE")
	apply := func(name string) {
		t.Helper()
		migration, err := os.ReadFile("../../../scripts/migration/postgres/" + name)
		must(err)
		must(db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("SET LOCAL search_path TO " + schemaName).Error; err != nil {
				return err
			}
			return tx.Exec(string(migration)).Error
		}))
	}

	apply("0009_user_following.up.sql")
	table := schemaName + ".p_following"
	must(db.Exec("INSERT INTO " + table + ` (user_id, follow_id, is_del) VALUES
		(1, 2, 1), (1, 2, 0), (1, 2, 0), (2, 1, 0), (3, 2, 1)`).Error)
	apply("0037_user_following_unique.up.sql")
	var ids []int64
	must(db.Unscoped().Model(&dbr.Following{}).Order("id").Pluck("id", &ids).Error)
	if !slices.Equal(ids, []int64{1, 2, 4, 5}) {
		t.Fatalf("migration retained follow IDs %v, want [1 2 4 5]", ids)
	}

	follows := newFollowingManageService(db)
	if follows.IsFollow(3, 2) {
		t.Fatal("soft-deleted follow was reported as active")
	}
	const requests = 8
	start := make(chan struct{})
	results := make(chan error, requests)
	for range requests {
		go func() {
			<-start
			results <- follows.FollowUser(3, 2)
		}()
	}
	close(start)
	for range requests {
		if err := <-results; err != nil {
			t.Errorf("concurrent follow failed: %v", err)
		}
	}
	assertCounts := func(want int64) {
		t.Helper()
		followingCount, _, err := follows.GetFollowCount(3)
		must(err)
		_, followerCount, err := follows.GetFollowCount(2)
		must(err)
		if followingCount != want || followerCount != want+1 {
			t.Fatalf("following count = %d, follower count = %d, want %d and %d", followingCount, followerCount, want, want+1)
		}
	}
	assertCounts(1)
	must(follows.FollowUser(3, 2))
	assertCounts(1)
	must(follows.UnfollowUser(3, 2))
	assertCounts(0)
	must(follows.FollowUser(3, 2))
	assertCounts(1)

	apply("0037_user_following_unique.down.sql")
	must(db.Exec("INSERT INTO " + table + " (user_id, follow_id) VALUES (1, 2)").Error)
}
