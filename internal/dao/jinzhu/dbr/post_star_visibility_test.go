// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package dbr

import (
	"reflect"
	"sort"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	_ "modernc.org/sqlite"
)

// 星标列表可见性矩阵测试(issue #31): 可见性 × 访问者与帖子作者的关系 × 审核状态。
// 语义基准是 servants/base.CanViewTweet, dao 层在 SQL 中同源镜像:
//   - 作者本人/管理员/审核员放行(作者按行放行, 管理侧整表放行);
//   - 其余访问者(含游客)要求已过审, 且 公开 或 关注可见且已关注作者;
//   - 私密/好友可见对非作者/非管理侧一律拒绝(好友功能已移除, 无好友关系表)。
//
// 用例: 星标列表列出 starOwner(id=2) 的全部星标, 各访问者只能看到自己应见的帖子。
func TestPostStarListVisibilityMatrix(t *testing.T) {
	const (
		author    = 1 // 帖子作者
		starOwner = 2 // 星标列表所属用户(资料页所有者, 不等于帖子作者)
		follower  = 3 // 关注了 author 的用户
		stranger  = 4 // 与 author 无任何关系的用户
		adminID   = 5
		auditorID = 6
	)

	db, err := gorm.Open(&sqlite.Dialector{
		DriverName: "sqlite",
		DSN:        "file:" + t.Name() + "?mode=memory&cache=shared",
	}, &gorm.Config{NamingStrategy: schema.NamingStrategy{SingularTable: true}})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err = db.AutoMigrate(&Post{}, &PostStar{}, &Following{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	posts := []struct {
		id         int64
		authorID   int64
		visibility PostVisibleT
		audit      PostAuditT
	}{
		{id: 1, authorID: author, visibility: PostVisitPublic, audit: PostAuditApproved},
		{id: 2, authorID: author, visibility: PostVisitPublic, audit: PostAuditPending},
		{id: 3, authorID: author, visibility: PostVisitFollowing, audit: PostAuditApproved},
		{id: 4, authorID: author, visibility: PostVisitFollowing, audit: PostAuditPending},
		{id: 5, authorID: author, visibility: PostVisitFriend, audit: PostAuditApproved},
		{id: 6, authorID: author, visibility: PostVisitPrivate, audit: PostAuditApproved},
		{id: 7, authorID: starOwner, visibility: PostVisitPrivate, audit: PostAuditPending},
		{id: 8, authorID: follower, visibility: PostVisitPublic, audit: PostAuditApproved},
		{id: 9, authorID: author, visibility: 70 /* 保留值: CanViewTweet default 拒绝 */, audit: PostAuditApproved},
		{id: 10, authorID: stranger, visibility: PostVisitFriend, audit: PostAuditApproved},
	}
	for _, p := range posts {
		post := &Post{Model: &Model{ID: p.id}, UserID: p.authorID, Visibility: p.visibility, AuditStatus: p.audit}
		if err = db.Create(post).Error; err != nil {
			t.Fatalf("create post %d: %v", p.id, err)
		}
		// 全部由 starOwner 星标
		star := &PostStar{Model: &Model{ID: 100 + p.id}, PostID: p.id, UserID: starOwner}
		if err = db.Omit("Post").Create(star).Error; err != nil {
			t.Fatalf("create star %d: %v", p.id, err)
		}
	}
	// follower 关注 author (following 表行: user_id=关注者 follow_id=被关注者, 与 Ds.IsFollow 口径一致)
	if err = db.Omit("User").Create(&Following{Model: &Model{ID: 1}, UserId: follower, FollowId: author}).Error; err != nil {
		t.Fatalf("create following: %v", err)
	}

	user := func(id int64, admin bool, roles string) *User {
		return &User{Model: &Model{ID: id}, IsAdmin: admin, Roles: roles}
	}

	cases := []struct {
		name   string
		viewer *User   // nil/ID=0 视为游客
		want   []int64 // 期望可见的帖子 id(升序)
	}{
		{name: "guest", viewer: nil, want: []int64{1, 8}},
		{name: "empty_viewer_as_guest", viewer: &User{Model: &Model{ID: 0}}, want: []int64{1, 8}},
		// 星标所有者本人: 仅作者维度放行(自己那篇私密待审帖), 关注可见帖未关注作者则不可见
		{name: "star_owner", viewer: user(starOwner, false, ""), want: []int64{1, 7, 8}},
		{name: "follower_of_author", viewer: user(follower, false, ""), want: []int64{1, 3, 8}},
		{name: "stranger", viewer: user(stranger, false, ""), want: []int64{1, 8, 10}},
		// 帖子作者看自己的帖子: 可见性/审核一律放行(镜像 CanViewTweet 作者豁免)
		{name: "post_author", viewer: user(author, false, ""), want: []int64{1, 2, 3, 4, 5, 6, 8, 9}},
		{name: "admin", viewer: user(adminID, true, ""), want: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
		{name: "auditor", viewer: user(auditorID, false, RoleAuditor), want: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
	}

	conds := &ConditionsT{"ORDER": db.NamingStrategy.TableName("PostStar") + ".id DESC"}
	star := &PostStar{UserID: starOwner}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := star.List(db, conds, tc.viewer, 0, 0)
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			got := make([]int64, 0, len(res))
			for _, s := range res {
				if s.Post == nil {
					t.Fatalf("star id=%d 缺少关联 Post(Joins 未生效)", s.ID)
				}
				got = append(got, s.Post.ID)
			}
			sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("List() visible posts = %v, want %v", got, tc.want)
			}
			// Count 必须与 List 同口径, 否则分页 total 泄露被隐藏帖子数量
			total, err := star.Count(db, tc.viewer, &ConditionsT{})
			if err != nil {
				t.Fatalf("Count() error = %v", err)
			}
			if int(total) != len(tc.want) {
				t.Errorf("Count() = %d, want %d", total, len(tc.want))
			}
		})
	}

	t.Run("ordered_desc_by_post_id", func(t *testing.T) {
		res, err := star.List(db, conds, nil, 0, 0)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		var got []int64
		for _, s := range res {
			got = append(got, s.Post.ID)
		}
		if !reflect.DeepEqual(got, []int64{8, 1}) {
			t.Errorf("List() order = %v, want [8 1] (post id 降序)", got)
		}
	})

	t.Run("star_owner_scope", func(t *testing.T) {
		// 列表只应包含 starOwner 的星标: 补一条 stranger 的星标确认过滤仍生效
		if err = db.Omit("Post").Create(&PostStar{Model: &Model{ID: 999}, PostID: 6, UserID: stranger}).Error; err != nil {
			t.Fatalf("create extra star: %v", err)
		}
		res, err := star.List(db, conds, nil, 0, 0)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		for _, s := range res {
			if s.UserID != starOwner {
				t.Errorf("List() 返回了他人星标 star.user_id=%d", s.UserID)
			}
		}
	})
}
