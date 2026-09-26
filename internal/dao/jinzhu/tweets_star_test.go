// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jinzhu

import (
	"reflect"
	"sort"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	_ "modernc.org/sqlite"
)

// newStarTestDB 构造无 config.yaml 依赖的内存库(仅依赖 gorm 自身的 NamingStrategy)
func newStarTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(&sqlite.Dialector{
		DriverName: "sqlite",
		DSN:        "file:" + t.Name() + "?mode=memory&cache=shared",
	}, &gorm.Config{NamingStrategy: schema.NamingStrategy{SingularTable: true}})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err = db.AutoMigrate(&dbr.User{}, &dbr.Post{}, &dbr.PostStar{}, &dbr.Following{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

// TestStarListViewerResolvesAdminBypass 校验"我的星标"自查路径(GetUserPostStars/GetUserPostStarCount)
// 会补查 is_admin/roles, 从而与 CanViewTweet 的管理侧豁免同源; 普通用户与查不到的 uid 不放宽。
func TestStarListViewerResolvesAdminBypass(t *testing.T) {
	db := newStarTestDB(t)

	users := []*dbr.User{
		{Model: &dbr.Model{ID: 1}, Username: "author"},
		{Model: &dbr.Model{ID: 2}, Username: "normal"},
		{Model: &dbr.Model{ID: 5}, Username: "admin", IsAdmin: true},
		{Model: &dbr.Model{ID: 6}, Username: "auditor", Roles: dbr.RoleAuditor},
	}
	for _, u := range users {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("create user %s: %v", u.Username, err)
		}
	}

	s := &tweetSrv{db: db}
	cases := []struct {
		uid         int64
		wantAdmin   bool
		wantAuditor bool
	}{
		{uid: 2, wantAdmin: false, wantAuditor: false},
		{uid: 5, wantAdmin: true, wantAuditor: false},
		{uid: 6, wantAdmin: false, wantAuditor: true},
		{uid: 999, wantAdmin: false, wantAuditor: false}, // 用户不存在: 降级为仅作者维度, 不放宽
	}
	for _, tc := range cases {
		viewer := s.starListViewer(tc.uid)
		if viewer.ID != tc.uid {
			t.Errorf("starListViewer(%d).ID = %d, want %d", tc.uid, viewer.ID, tc.uid)
		}
		if viewer.IsAdmin != tc.wantAdmin {
			t.Errorf("starListViewer(%d).IsAdmin = %v, want %v", tc.uid, viewer.IsAdmin, tc.wantAdmin)
		}
		if got := viewer.HasRole(dbr.RoleAuditor); got != tc.wantAuditor {
			t.Errorf("starListViewer(%d).HasRole(auditor) = %v, want %v", tc.uid, got, tc.wantAuditor)
		}
	}
}

// TestGetUserPostStarsSelfPath 以矩阵口径校验自查路径的列表与计数:
// 星标所有者只能看到自己应见的帖子, 管理员/审核员走 CanViewTweet 的豁免。
func TestGetUserPostStarsSelfPath(t *testing.T) {
	db := newStarTestDB(t)

	users := []*dbr.User{
		{Model: &dbr.Model{ID: 1}, Username: "author"},
		{Model: &dbr.Model{ID: 2}, Username: "normal"},
		{Model: &dbr.Model{ID: 5}, Username: "admin", IsAdmin: true},
		{Model: &dbr.Model{ID: 6}, Username: "auditor", Roles: dbr.RoleAuditor},
	}
	for _, u := range users {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("create user %s: %v", u.Username, err)
		}
	}

	posts := []struct {
		id       int64
		authorID int64
		vis      dbr.PostVisibleT
		audit    dbr.PostAuditT
	}{
		{id: 1, authorID: 1, vis: dbr.PostVisitPublic, audit: dbr.PostAuditApproved},
		{id: 2, authorID: 1, vis: dbr.PostVisitFriend, audit: dbr.PostAuditApproved},
		{id: 3, authorID: 1, vis: dbr.PostVisitPublic, audit: dbr.PostAuditPending},
		{id: 4, authorID: 2, vis: dbr.PostVisitPrivate, audit: dbr.PostAuditPending},
	}
	for _, p := range posts {
		if err := db.Create(&dbr.Post{Model: &dbr.Model{ID: p.id}, UserID: p.authorID, Visibility: p.vis, AuditStatus: p.audit}).Error; err != nil {
			t.Fatalf("create post %d: %v", p.id, err)
		}
	}

	// 普通用户(2)星标全部 4 篇: 只能看见 公开已过审 + 自己作者维度的私密待审帖
	for _, p := range posts {
		star := &dbr.PostStar{Model: &dbr.Model{ID: 100 + p.id}, PostID: p.id, UserID: 2}
		if err := db.Omit("Post").Create(star).Error; err != nil {
			t.Fatalf("create star %d: %v", p.id, err)
		}
	}
	// 管理员(5)与审核员(6)各自星标 好友可见 与 待审公开帖: 豁免应放行
	for _, uid := range []int64{5, 6} {
		for _, pid := range []int64{2, 3} {
			star := &dbr.PostStar{Model: &dbr.Model{ID: uid*10 + pid}, PostID: pid, UserID: uid}
			if err := db.Omit("Post").Create(star).Error; err != nil {
				t.Fatalf("create star user=%d post=%d: %v", uid, pid, err)
			}
		}
	}

	s := &tweetSrv{db: db}
	cases := []struct {
		uid  int64
		want []int64
	}{
		{uid: 2, want: []int64{1, 4}},
		{uid: 5, want: []int64{2, 3}},
		{uid: 6, want: []int64{2, 3}},
	}
	for _, tc := range cases {
		res, err := s.GetUserPostStars(tc.uid, 10, 0)
		if err != nil {
			t.Fatalf("GetUserPostStars(%d) error = %v", tc.uid, err)
		}
		got := make([]int64, 0, len(res))
		for _, star := range res {
			if star.Post == nil {
				t.Fatalf("star id=%d 缺少关联 Post", star.ID)
			}
			got = append(got, star.Post.ID)
		}
		sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("GetUserPostStars(%d) = %v, want %v", tc.uid, got, tc.want)
		}

		total, err := s.GetUserPostStarCount(tc.uid)
		if err != nil {
			t.Fatalf("GetUserPostStarCount(%d) error = %v", tc.uid, err)
		}
		if int(total) != len(tc.want) {
			t.Errorf("GetUserPostStarCount(%d) = %d, want %d", tc.uid, total, len(tc.want))
		}
	}
}
