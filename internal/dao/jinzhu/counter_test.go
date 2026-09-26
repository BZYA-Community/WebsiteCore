// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jinzhu

import (
	"sync"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"gorm.io/plugin/soft_delete"
	_ "modernc.org/sqlite"
)

// nopCacheIndexSrv 测试用空索引缓存服务
type nopCacheIndexSrv struct{}

func (nopCacheIndexSrv) SendAction(core.IdxAct, *dbr.Post) {}

func newCounterTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(&sqlite.Dialector{DriverName: "sqlite", DSN: dsn}, &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "p_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&dbr.Post{}, &dbr.UserMetric{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return db
}

func createCounterTestPost(t *testing.T, db *gorm.DB, commentCount int64) *dbr.Post {
	t.Helper()
	post := &dbr.Post{
		Model:        &dbr.Model{},
		UserID:       1,
		CommentCount: commentCount,
		UpvoteCount:  2,
	}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("Create post error = %v", err)
	}
	return post
}

// TestIncPostCounterUpdatesAtomically 验证计数列由SQL表达式原子更新 且能一并写入附属列
func TestIncPostCounterUpdatesAtomically(t *testing.T) {
	db := newCounterTestDB(t)
	srv := &tweetManageSrv{db: db, cacheIndex: nopCacheIndexSrv{}}
	post := createCounterTestPost(t, db, 5)

	if err := srv.IncPostCounter(post, "comment_count", 1, 12345); err != nil {
		t.Fatalf("IncPostCounter() error = %v", err)
	}

	var got dbr.Post
	if err := db.Where("id = ?", post.ID).First(&got).Error; err != nil {
		t.Fatalf("reload post error = %v", err)
	}
	if got.CommentCount != 6 {
		t.Fatalf("CommentCount = %d, want 6", got.CommentCount)
	}
	if got.LatestRepliedOn != 12345 {
		t.Fatalf("LatestRepliedOn = %d, want 12345", got.LatestRepliedOn)
	}
	// 内存计数需同步 保证索引推送/度量更新使用最新值
	if post.CommentCount != 6 {
		t.Fatalf("in-memory CommentCount = %d, want 6", post.CommentCount)
	}
	if post.LatestRepliedOn != 12345 {
		t.Fatalf("in-memory LatestRepliedOn = %d, want 12345", post.LatestRepliedOn)
	}
	if got.UpvoteCount != 2 {
		t.Fatalf("UpvoteCount = %d, want 2 (计数更新不应影响其它列)", got.UpvoteCount)
	}

	// 减量
	if err := srv.IncPostCounter(post, "comment_count", -2, 0); err != nil {
		t.Fatalf("IncPostCounter() decrement error = %v", err)
	}
	if err := db.Where("id = ?", post.ID).First(&got).Error; err != nil {
		t.Fatalf("reload post error = %v", err)
	}
	if got.CommentCount != 4 {
		t.Fatalf("CommentCount = %d, want 4", got.CommentCount)
	}

	// 非计数列被拒绝
	if err := srv.IncPostCounter(post, "user_id", 1, 0); err == nil {
		t.Fatal("IncPostCounter() with non-counter column want error, got nil")
	}
}

// TestIncPostCounterDoesNotOverwriteConcurrentEdit 验证只更新计数列
// 不会像全行 Save 那样用内存旧值覆盖其它列的并发修改
func TestIncPostCounterDoesNotOverwriteConcurrentEdit(t *testing.T) {
	db := newCounterTestDB(t)
	srv := &tweetManageSrv{db: db, cacheIndex: nopCacheIndexSrv{}}
	post := createCounterTestPost(t, db, 5) // 内存中持旧值

	// 模拟并发请求先修改了其它列
	if err := db.Model(&dbr.Post{}).Where("id = ?", post.ID).Updates(map[string]any{
		"upvote_count": 99,
		"is_lock":      1,
	}).Error; err != nil {
		t.Fatalf("concurrent update error = %v", err)
	}

	if err := srv.IncPostCounter(post, "comment_count", 1, 0); err != nil {
		t.Fatalf("IncPostCounter() error = %v", err)
	}

	var got dbr.Post
	if err := db.Where("id = ?", post.ID).First(&got).Error; err != nil {
		t.Fatalf("reload post error = %v", err)
	}
	if got.CommentCount != 6 {
		t.Fatalf("CommentCount = %d, want 6", got.CommentCount)
	}
	if got.UpvoteCount != 99 {
		t.Fatalf("UpvoteCount = %d, want 99 (并发修改被旧值覆盖)", got.UpvoteCount)
	}
	if got.IsLock != 1 {
		t.Fatalf("IsLock = %d, want 1 (并发修改被旧值覆盖)", got.IsLock)
	}
}

// TestIncPostCounterSkipsSoftDeletedPost 验证软删除(is_del=1)的帖子不会被更新
func TestIncPostCounterSkipsSoftDeletedPost(t *testing.T) {
	db := newCounterTestDB(t)
	srv := &tweetManageSrv{db: db, cacheIndex: nopCacheIndexSrv{}}
	post := &dbr.Post{
		Model:        &dbr.Model{IsDel: soft_delete.DeletedAt(1)},
		UserID:       1,
		CommentCount: 3,
	}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("Create soft-deleted post error = %v", err)
	}

	if err := srv.IncPostCounter(post, "comment_count", 1, 0); err != nil {
		t.Fatalf("IncPostCounter() error = %v", err)
	}

	var got dbr.Post
	if err := db.Unscoped().Where("id = ?", post.ID).First(&got).Error; err != nil {
		t.Fatalf("reload post error = %v", err)
	}
	if got.CommentCount != 3 {
		t.Fatalf("CommentCount = %d, want 3 (软删除记录不应被更新)", got.CommentCount)
	}
}

// TestIncPostCounterConcurrentIncrements 并发自增不丢计数
// 每个goroutine持有各自的内存副本(等价于并发请求各自读库后的写回)
func TestIncPostCounterConcurrentIncrements(t *testing.T) {
	db := newCounterTestDB(t)
	// sqlite单连接串行化 避免测试环境锁库
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB() error = %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	srv := &tweetManageSrv{db: db, cacheIndex: nopCacheIndexSrv{}}
	post := createCounterTestPost(t, db, 0)

	const n = 32
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := &dbr.Post{Model: &dbr.Model{ID: post.ID}}
			if xerr := srv.IncPostCounter(p, "comment_count", 1, 0); xerr != nil {
				errs <- xerr
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent IncPostCounter() error = %v", err)
	}

	var got dbr.Post
	if err := db.Where("id = ?", post.ID).First(&got).Error; err != nil {
		t.Fatalf("reload post error = %v", err)
	}
	if got.CommentCount != n {
		t.Fatalf("CommentCount = %d, want %d (并发自增丢计数)", got.CommentCount, n)
	}
}

// TestUpdateUserMetricAtomicIncrement 验证站点指标计数由SQL表达式原子增减且不为负
func TestUpdateUserMetricAtomicIncrement(t *testing.T) {
	db := newCounterTestDB(t)
	srv := newUserMetricServentA(db)

	// 记录不存在时补建
	if err := srv.UpdateUserMetric(7, cs.MetricActionCreateTweet); err != nil {
		t.Fatalf("UpdateUserMetric(create) error = %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := srv.UpdateUserMetric(7, cs.MetricActionCreateTweet); err != nil {
			t.Fatalf("UpdateUserMetric(create) error = %v", err)
		}
	}

	metric := &dbr.UserMetric{}
	if err := db.Where("user_id = ?", 7).First(metric).Error; err != nil {
		t.Fatalf("reload user metric error = %v", err)
	}
	if metric.TweetsCount != 4 {
		t.Fatalf("TweetsCount = %d, want 4", metric.TweetsCount)
	}
	if metric.LatestTrendsOn <= 0 {
		t.Fatalf("LatestTrendsOn = %d, want > 0", metric.LatestTrendsOn)
	}

	// 删除自减 且不会减成负数
	for i := 0; i < 6; i++ {
		if err := srv.UpdateUserMetric(7, cs.MetricActionDeleteTweet); err != nil {
			t.Fatalf("UpdateUserMetric(delete) error = %v", err)
		}
	}
	if err := db.Where("user_id = ?", 7).First(metric).Error; err != nil {
		t.Fatalf("reload user metric error = %v", err)
	}
	if metric.TweetsCount != 0 {
		t.Fatalf("TweetsCount = %d, want 0", metric.TweetsCount)
	}
}
