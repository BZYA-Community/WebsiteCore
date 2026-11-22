package jinzhu

import (
	"slices"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
)

func TestUserFollowingUniqueness(t *testing.T) {
	runner, conn, db := newPostgresMigrationTest(t)
	ctx := t.Context()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	must(runner.Migrate(36))
	_, err := conn.ExecContext(ctx, `INSERT INTO p_following (user_id, follow_id, is_del) VALUES
		(1, 2, 1), (1, 2, 0), (1, 2, 0), (2, 1, 0), (3, 2, 1)`)
	must(err)
	must(runner.Migrate(37))
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

	must(runner.Migrate(36))
	_, err = conn.ExecContext(ctx, "INSERT INTO p_following (user_id, follow_id) VALUES (1, 2)")
	must(err)
}
