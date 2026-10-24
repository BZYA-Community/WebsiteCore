package jinzhu

import (
	"strings"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestProjectVisibleScopeCarriesPostReadRestrictions(t *testing.T) {
	db, err := gorm.Open(postgres.Open("host=localhost user=test dbname=test sslmode=disable"), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		NamingStrategy:       schema.NamingStrategy{TablePrefix: projectTablePrefix(), SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	viewer := func(id int64, permissions ...string) *ms.User {
		return &ms.User{Model: &ms.Model{ID: id}, Status: ms.UserStatusNormal, Permissions: permissions}
	}
	query := func(user *ms.User) string {
		var count int64
		sql := projectVisibleScope(db, user).Count(&count).Statement.SQL.String()
		if strings.Count(sql, "(") != strings.Count(sql, ")") {
			t.Fatalf("project query has unbalanced parentheses: %s", sql)
		}
		return sql
	}

	guestSQL := query(nil)
	for _, clause := range []string{"post.audit_status", "post.visibility"} {
		if !strings.Contains(guestSQL, clause) {
			t.Fatalf("guest project query omitted %s: %s", clause, guestSQL)
		}
	}

	ordinarySQL := query(viewer(2, authz.PostView))
	for _, clause := range []string{"post.user_id", "post.audit_status", "post.visibility", "following"} {
		if !strings.Contains(ordinarySQL, clause) {
			t.Fatalf("authenticated project query omitted %s: %s", clause, ordinarySQL)
		}
	}
	if strings.Contains(ordinarySQL, "review_task") {
		t.Fatalf("ordinary viewer unexpectedly received review access: %s", ordinarySQL)
	}

	reviewerSQL := query(viewer(3, authz.PostView, authz.ContentReview))
	if !strings.Contains(reviewerSQL, "review_task") || !strings.Contains(reviewerSQL, "assignee_id") {
		t.Fatalf("reviewer project query omitted assignment restriction: %s", reviewerSQL)
	}

	revokedSQL := query(viewer(4, authz.ContentReview))
	if !strings.Contains(revokedSQL, "1 = 0") {
		t.Fatalf("viewer without post.view was not denied: %s", revokedSQL)
	}
}
