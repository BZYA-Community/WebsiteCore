package jinzhu

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// 安全审计02 中-1/低-2 的回归测试:
//   - 待审核帖经"作者自赞 + 公开点赞列表"不得泄露给游客/他人;
//   - 未过审帖子(含作者本人)禁止新增点赞/收藏, 取消不受限;
//   - 取消关注后, 关注可见帖不得残留在自己的点赞/收藏列表。
func TestStarCollectionVisibilityScope(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; PostgreSQL star/collection visibility integration test")
	}
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		t.Fatal(err)
	}
	prefix := "ut_star_" + hex.EncodeToString(bytes) + "_"
	// Following.User 的关系定义会在 user 表上建指向 following(follow_id) 的外键,
	// 而 follow_id 非唯一键, 测试建表时禁用外键约束(生产库亦不依赖该约束)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy:                           schema.NamingStrategy{TablePrefix: prefix, SingularTable: true},
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	models := []any{&dbr.Following{}, &dbr.User{}, &dbr.IdentityGroup{}, &dbr.IdentityGroupPermission{}, &dbr.UserIdentityGroup{},
		&dbr.Post{}, &dbr.PostMetric{}, &dbr.PostStar{}, &dbr.PostCollection{}}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for i := len(models) - 1; i >= 0; i-- {
			if err := db.Migrator().DropTable(models[i]); err != nil {
				t.Log(err)
			}
		}
	})
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	group := func(key string, permissions ...string) *dbr.IdentityGroup {
		t.Helper()
		g := &dbr.IdentityGroup{Key: key, Name: key}
		must(db.Create(g).Error)
		for _, permission := range permissions {
			must(db.Create(&dbr.IdentityGroupPermission{GroupID: g.ID, Permission: permission}).Error)
		}
		return g
	}
	// lockIdentityPolicy 会锁定 key=guest 的组行, 必须存在
	userGroup := group("guest", authz.PostView, authz.PostCreate, authz.CommunityInteract)
	user := func(name string) *ms.User {
		t.Helper()
		u := &ms.User{Model: &ms.Model{}, Username: name, Nickname: name, Status: ms.UserStatusNormal}
		must(db.Create(u).Error)
		must(db.Create(&dbr.UserIdentityGroup{UserID: u.ID, GroupID: userGroup.ID}).Error)
		return u
	}
	author, fan, stranger := user("author"), user("fan"), user("stranger")
	post := func(visibility ms.PostVisibleT, audit ms.PostAuditT) *ms.Post {
		t.Helper()
		p := &ms.Post{Model: &ms.Model{}, UserID: author.ID, Visibility: visibility, AuditStatus: audit}
		must(db.Create(p).Error)
		return p
	}
	pendingPost := post(ms.PostVisitPublic, ms.PostAuditPending)
	approvedPost := post(ms.PostVisitPublic, ms.PostAuditApproved)
	followPost := post(ms.PostVisitFollowing, ms.PostAuditApproved)

	manage := &tweetManageSrv{db: db}
	tweets := &tweetSrv{db: db}

	// 中-1 写入口径: 未过审帖子禁止新增点赞/收藏(含作者本人)
	if _, err := manage.CreatePostStar(pendingPost.ID, author.ID); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("author star on pending post: %v", err)
	}
	if _, err := manage.CreatePostCollection(pendingPost.ID, author.ID); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("author collection on pending post: %v", err)
	}
	if _, err := manage.CreatePostStar(pendingPost.ID, stranger.ID); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("stranger star on pending post: %v", err)
	}

	// 历史遗留数据: 作者此前给自己的待审帖点过赞(绕过写入口径直接落库)
	must(db.Create(&dbr.PostStar{Model: &dbr.Model{}, PostID: pendingPost.ID, UserID: author.ID}).Error)

	// 中-1 读口径: 游客看作者的点赞列表, 不得看到待审帖
	res, total, err := tweets.ListUserStarTweets(&cs.VistUser{UserId: author.ID, RelTyp: cs.RelationGuest}, 10, 0)
	must(err)
	if total != 0 || len(res) != 0 {
		t.Fatalf("guest saw pending post in author star list: total=%d len=%d", total, len(res))
	}
	// 本人视角仍可见(作者本就有权读自己的待审帖)
	res, total, err = tweets.ListUserStarTweets(&cs.VistUser{UserId: author.ID, RelTyp: cs.RelationSelf}, 10, 0)
	must(err)
	if total != 1 || len(res) != 1 || res[0].PostID != pendingPost.ID {
		t.Fatalf("self star list lost own pending post: total=%d len=%d", total, len(res))
	}

	// fan 关注 author 后点赞/收藏关注可见帖与公开已过审帖
	follows := newFollowingManageService(db)
	must(follows.FollowUser(fan.ID, author.ID))
	if _, err := manage.CreatePostStar(followPost.ID, fan.ID); err != nil {
		t.Fatalf("fan star on followed post: %v", err)
	}
	if _, err := manage.CreatePostStar(approvedPost.ID, fan.ID); err != nil {
		t.Fatalf("fan star on approved post: %v", err)
	}
	if _, err := manage.CreatePostCollection(followPost.ID, fan.ID); err != nil {
		t.Fatalf("fan collection on followed post: %v", err)
	}
	// 未关注者不得操作关注可见帖(既有行为, 防回归)
	if _, err := manage.CreatePostStar(followPost.ID, stranger.ID); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("stranger star on follow-visible post: %v", err)
	}

	stars, err := tweets.GetUserPostStars(fan.ID, 10, 0)
	must(err)
	if len(stars) != 2 {
		t.Fatalf("fan stars while following: %d", len(stars))
	}
	starCount, err := tweets.GetUserPostStarCount(fan.ID)
	must(err)
	if starCount != 2 {
		t.Fatalf("fan star count while following: %d", starCount)
	}
	collections, err := tweets.GetUserPostCollections(fan.ID, 0, 10)
	must(err)
	if len(collections) != 1 {
		t.Fatalf("fan collections while following: %d", len(collections))
	}

	// 低-2: 取关后关注可见帖不得残留在自己的点赞/收藏列表
	must(follows.UnfollowUser(fan.ID, author.ID))
	stars, err = tweets.GetUserPostStars(fan.ID, 10, 0)
	must(err)
	if len(stars) != 1 || stars[0].PostID != approvedPost.ID {
		t.Fatalf("fan stars after unfollow: %d", len(stars))
	}
	starCount, err = tweets.GetUserPostStarCount(fan.ID)
	must(err)
	if starCount != 1 {
		t.Fatalf("fan star count after unfollow: %d", starCount)
	}
	collections, err = tweets.GetUserPostCollections(fan.ID, 0, 10)
	must(err)
	if len(collections) != 0 {
		t.Fatalf("fan collections after unfollow: %d", len(collections))
	}
	collectionCount, err := tweets.GetUserPostCollectionCount(fan.ID)
	must(err)
	if collectionCount != 0 {
		t.Fatalf("fan collection count after unfollow: %d", collectionCount)
	}

	// 取消历史遗留的待审帖自赞不受写入口径限制
	must(manage.DeletePostStar(&ms.PostStar{PostID: pendingPost.ID, UserID: author.ID}))
}
