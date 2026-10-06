package base

import (
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

type visibilityData struct {
	core.DataService
	follows bool
}

func (d *visibilityData) IsFollow(int64, int64) bool { return d.follows }

func TestTweetVisibilityDoesNotInferPermissionsFromRoles(t *testing.T) {
	for _, tt := range []struct {
		name                           string
		visibility                     ms.PostVisibleT
		status                         ms.PostAuditT
		permissions                    []string
		owner, follows, operator, want bool
	}{
		{name: "public", visibility: core.PostVisitPublic, status: ms.PostAuditApproved, permissions: []string{"post.view"}, want: true},
		{name: "legacy admin is not authority", visibility: core.PostVisitPrivate, status: ms.PostAuditApproved, permissions: []string{"post.view"}},
		{name: "review pending public", visibility: core.PostVisitPublic, status: ms.PostAuditPending, permissions: []string{"post.view", "content.review"}, want: true},
		{name: "review cannot read private", visibility: core.PostVisitPrivate, status: ms.PostAuditApproved, permissions: []string{"post.view", "content.review"}},
		{name: "review cannot bypass follows", visibility: core.PostVisitFollowing, status: ms.PostAuditPending, permissions: []string{"post.view", "content.review"}},
		{name: "review followed author", visibility: core.PostVisitFollowing, status: ms.PostAuditPending, permissions: []string{"post.view", "content.review"}, follows: true, want: true},
		{name: "private permission", visibility: core.PostVisitPrivate, status: ms.PostAuditPending, permissions: []string{"post.view", "content.view_private"}, want: true},
		{name: "owner", visibility: core.PostVisitPrivate, status: ms.PostAuditPending, permissions: []string{"post.view"}, owner: true, want: true},
		{name: "owner view revoked", visibility: core.PostVisitPrivate, status: ms.PostAuditApproved, owner: true},
		{name: "operator", visibility: core.PostVisitPrivate, status: ms.PostAuditPending, operator: true, want: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			user := &ms.User{Model: &ms.Model{ID: 1}, Status: ms.UserStatusNormal, IsAdmin: true, Roles: "admin", Permissions: tt.permissions, IsOperator: tt.operator}
			post := &ms.Post{Model: &ms.Model{ID: 3}, UserID: 2, Visibility: tt.visibility, AuditStatus: tt.status}
			if tt.owner {
				post.UserID = user.ID
			}
			s := &DaoServant{Ds: &visibilityData{follows: tt.follows}}
			if got := s.CanViewTweet(user, post); got != tt.want {
				t.Fatalf("CanViewTweet=%v, want %v", got, tt.want)
			}
		})
	}
	s := &DaoServant{}
	if s.CanViewTweet(nil, (*ms.Post)(nil)) || s.CanViewTweet(nil, &ms.Post{AuditStatus: ms.PostAuditPending, Visibility: core.PostVisitPublic}) {
		t.Fatal("anonymous read exposed missing or pending content")
	}
}
