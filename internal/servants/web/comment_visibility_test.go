package web

import (
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

func TestCanReplyToComment(t *testing.T) {
	author := &ms.User{Model: &ms.Model{ID: 10}}
	stranger := &ms.User{Model: &ms.Model{ID: 20}}
	auditor := &ms.User{Model: &ms.Model{ID: 30}, Roles: ms.RoleAuditor}
	admin := &ms.User{Model: &ms.Model{ID: 40}, IsAdmin: true}
	tests := []struct {
		name   string
		status ms.PostAuditT
		viewer *ms.User
		want   bool
	}{
		{name: "approved stranger", status: ms.PostAuditApproved, viewer: stranger, want: true},
		{name: "pending author", status: ms.PostAuditPending, viewer: author, want: true},
		{name: "pending stranger", status: ms.PostAuditPending, viewer: stranger, want: false},
		{name: "rejected stranger", status: ms.PostAuditRejected, viewer: stranger, want: false},
		{name: "pending auditor", status: ms.PostAuditPending, viewer: auditor, want: true},
		{name: "pending admin", status: ms.PostAuditPending, viewer: admin, want: true},
		{name: "pending anonymous", status: ms.PostAuditPending, viewer: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canReplyToComment(tt.status, author.ID, tt.viewer); got != tt.want {
				t.Fatalf("canReplyToComment() = %v, want %v", got, tt.want)
			}
		})
	}
}
