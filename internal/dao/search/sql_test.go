package search

import (
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSQLSearchFiltersBeforePagination(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&dbr.Post{}, &dbr.PostContent{}); err != nil {
		t.Fatal(err)
	}
	posts := []*dbr.Post{
		{Model: &dbr.Model{ID: 1}, UserID: 7, Visibility: core.PostVisitPublic, AuditStatus: ms.PostAuditApproved, Tags: "Go,中文"},
		{Model: &dbr.Model{ID: 2}, UserID: 7, Visibility: core.PostVisitPrivate, AuditStatus: ms.PostAuditApproved},
		{Model: &dbr.Model{ID: 3}, UserID: 8, Visibility: core.PostVisitPublic, AuditStatus: ms.PostAuditPending},
		{Model: &dbr.Model{ID: 4, IsDel: 1}, Visibility: core.PostVisitPublic, AuditStatus: ms.PostAuditApproved},
	}
	for _, p := range posts {
		if err = db.Create(p).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []int64{1, 2, 3, 4} {
		c := &dbr.PostContent{Model: &dbr.Model{}, PostID: id, Type: dbr.ContentTypeText, Content: "Hello 中文 100%"}
		if err = db.Create(c).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &sqlTweetSearchServant{db: db}
	for _, tc := range []struct {
		name  string
		user  *ms.User
		query string
		kind  core.SearchType
		total int64
	}{
		{"guest", nil, "hello", core.SearchTypeDefault, 1},
		{"author", &ms.User{Model: &dbr.Model{ID: 7}}, "中文", core.SearchTypeDefault, 2},
		{"other", &ms.User{Model: &dbr.Model{ID: 8}}, "", core.SearchTypeDefault, 1},
		{"admin", &ms.User{IsAdmin: true}, "", core.SearchTypeDefault, 2},
		{"tag", nil, "go", core.SearchTypeTag, 1},
		{"partial tag", nil, "g", core.SearchTypeTag, 0},
		{"literal percent", nil, "100%", core.SearchTypeDefault, 1},
		{"wildcard", nil, "_%", core.SearchTypeDefault, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := s.Search(tc.user, &core.QueryReq{Query: tc.query, Type: tc.kind}, 0, 1)
			if err != nil {
				t.Fatal(err)
			}
			if resp.Total != tc.total || len(resp.Items) != min(int(tc.total), 1) {
				t.Fatalf("unexpected result: %+v", resp)
			}
			page, err := s.Search(tc.user, &core.QueryReq{Query: tc.query, Type: tc.kind}, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			if page.Total != tc.total || len(page.Items) != max(0, int(tc.total)-1) {
				t.Fatalf("unexpected second page: %+v", page)
			}
		})
	}
}
