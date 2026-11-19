package jinzhu

import (
	"crypto/rand"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestTweetListTotalsExcludeSoftDeletedPosts(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; PostgreSQL post list integration test")
	}
	naming := schema.NamingStrategy{
		TablePrefix:   "ut_tweet_list_" + strings.ToLower(rand.Text()) + "_",
		SingularTable: true,
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{NamingStrategy: naming})
	must(err)
	sqlDB, err := db.DB()
	must(err)
	defer sqlDB.Close()

	db = db.Begin()
	must(db.Error)
	defer db.Rollback()

	oldPostTable, oldMetricTable := _post_, _post_metric_
	_post_, _post_metric_ = naming.TableName("Post"), naming.TableName("PostMetric")
	t.Cleanup(func() {
		_post_, _post_metric_ = oldPostTable, oldMetricTable
	})
	must(db.AutoMigrate(&dbr.Post{}, &dbr.PostMetric{}))

	newPost := func(visibility dbr.PostVisibleT, auditStatus dbr.PostAuditT) *dbr.Post {
		return &dbr.Post{Model: &dbr.Model{}, Visibility: visibility, AuditStatus: auditStatus}
	}
	postWithActiveMetric := newPost(dbr.PostVisitPublic, dbr.PostAuditApproved)
	postWithDeletedMetric := newPost(dbr.PostVisitPublic, dbr.PostAuditApproved)
	deletedPost := newPost(dbr.PostVisitPublic, dbr.PostAuditApproved)
	friendPost := newPost(dbr.PostVisitFriend, dbr.PostAuditApproved)
	pendingPost := newPost(dbr.PostVisitPublic, dbr.PostAuditPending)
	privatePost := newPost(dbr.PostVisitPrivate, dbr.PostAuditApproved)
	must(db.Create([]*dbr.Post{
		postWithActiveMetric, postWithDeletedMetric, deletedPost, friendPost, pendingPost, privatePost,
	}).Error)

	metricFor := func(post *dbr.Post) *dbr.PostMetric {
		return &dbr.PostMetric{Model: &dbr.Model{}, PostId: post.ID}
	}
	deletedMetric := metricFor(postWithDeletedMetric)
	must(db.Create([]*dbr.PostMetric{
		metricFor(postWithActiveMetric),
		deletedMetric,
		metricFor(deletedPost),
		metricFor(friendPost),
		metricFor(pendingPost),
		metricFor(privatePost),
	}).Error)
	must(deletedPost.Delete(db))
	must(deletedMetric.Delete(db))

	service := newTweetService(db)
	tests := []struct {
		name    string
		list    func(limit, offset int) ([]*dbr.Post, int64, error)
		wantIDs []int64
	}{
		{"newest", service.ListIndexNewestTweets, []int64{postWithActiveMetric.ID, postWithDeletedMetric.ID}},
		{"hot", service.ListIndexHotsTweets, []int64{postWithActiveMetric.ID}},
		{"search sync", service.ListSyncSearchTweets, []int64{postWithActiveMetric.ID, postWithDeletedMetric.ID, friendPost.ID}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limit := len(test.wantIDs)
			page, total, err := test.list(limit, 0)
			if err != nil {
				t.Fatal(err)
			}
			gotIDs := make([]int64, 0, len(page))
			for _, post := range page {
				gotIDs = append(gotIDs, post.ID)
			}
			slices.Sort(gotIDs)
			if total != int64(limit) || !slices.Equal(gotIDs, test.wantIDs) {
				t.Errorf("first page returned IDs %v and total %d, want IDs %v and total %d", gotIDs, total, test.wantIDs, limit)
			}

			page, total, err = test.list(limit, limit)
			if err != nil {
				t.Fatal(err)
			}
			if total != int64(limit) || len(page) != 0 {
				t.Errorf("after last page got %d posts and total %d, want 0 posts and total %d", len(page), total, limit)
			}
		})
	}
}
