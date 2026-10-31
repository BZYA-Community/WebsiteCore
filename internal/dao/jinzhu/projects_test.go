package jinzhu

import (
	"strings"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
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

func TestProjectAuditScopeUsesReviewAuthorization(t *testing.T) {
	db, err := gorm.Open(postgres.Open("host=localhost user=test dbname=test sslmode=disable"), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		NamingStrategy:       schema.NamingStrategy{TablePrefix: projectTablePrefix(), SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	query := func(actor *ms.User) string {
		var projects []*dbr.Project
		return projectAuditScope(db, actor).Where("project.post_id IN ?", []int64{7, 8}).Find(&projects).Statement.SQL.String()
	}
	reviewerSQL := query(&ms.User{Model: &ms.Model{ID: 42}, Status: ms.UserStatusNormal, Permissions: []string{authz.ContentReview}})
	for _, clause := range []string{"review_task", "assignee_id", "project.post_id"} {
		if !strings.Contains(reviewerSQL, clause) {
			t.Fatalf("reviewer audit query omitted %s: %s", clause, reviewerSQL)
		}
	}
	for _, clause := range []string{"post.audit_status", "post.visibility"} {
		if strings.Contains(reviewerSQL, clause) {
			t.Fatalf("audit query reused public visibility clause %s: %s", clause, reviewerSQL)
		}
	}
	viewAllSQL := query(&ms.User{Model: &ms.Model{ID: 43}, Status: ms.UserStatusNormal, Permissions: []string{authz.AuditViewAll}})
	if strings.Contains(viewAllSQL, "assignee_id") {
		t.Fatalf("view-all audit query was assignment-scoped: %s", viewAllSQL)
	}
}

func TestProjectFollowedScopeKeepsVisibilityAndRequiresViewer(t *testing.T) {
	db, err := gorm.Open(postgres.Open("host=localhost user=test dbname=test sslmode=disable"), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		NamingStrategy:       schema.NamingStrategy{TablePrefix: projectTablePrefix(), SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	viewer := &ms.User{Model: &ms.Model{ID: 42}, Status: ms.UserStatusNormal, Permissions: []string{authz.PostView}}
	query, order := projectFollowedScope(projectVisibleScope(db, viewer), viewer, true)
	var projects []*dbr.Project
	sql := query.Select("project.*").Order(order).Find(&projects).Statement.SQL.String()
	for _, clause := range []string{"post.audit_status", "post.visibility", "project_follow", "project_follow.user_id", "project_follow.created_on DESC"} {
		if !strings.Contains(sql, clause) {
			t.Fatalf("followed project query omitted %s: %s", clause, sql)
		}
	}

	guestQuery, guestOrder := projectFollowedScope(projectVisibleScope(db, nil), nil, true)
	guestSQL := guestQuery.Select("project.*").Order(guestOrder).Find(&projects).Statement.SQL.String()
	if !strings.Contains(guestSQL, "1 = 0") {
		t.Fatalf("anonymous followed-project query was not empty: %s", guestSQL)
	}
}
