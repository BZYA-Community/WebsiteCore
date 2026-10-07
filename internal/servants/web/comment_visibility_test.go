package web

import (
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

func TestCanReplyToComment(t *testing.T) {
	author := permissionUser(10, "comment.create")
	stranger := permissionUser(20, "comment.create")
	auditor := permissionUser(30, "comment.create", "audit.view_all")
	admin := permissionUser(40, "comment.create", "audit.view_all")
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
		{name: "rejected author", status: ms.PostAuditRejected, viewer: author, want: false},
		{name: "rejected auditor", status: ms.PostAuditRejected, viewer: auditor, want: false},
		{name: "rejected admin", status: ms.PostAuditRejected, viewer: admin, want: false},
		{name: "pending auditor", status: ms.PostAuditPending, viewer: auditor, want: true},
		{name: "pending admin", status: ms.PostAuditPending, viewer: admin, want: true},
		{name: "pending anonymous", status: ms.PostAuditPending, viewer: nil, want: false},
		{name: "approved anonymous", status: ms.PostAuditApproved, viewer: nil, want: false},
		{name: "approved revoked permission", status: ms.PostAuditApproved, viewer: permissionUser(20), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canReplyToComment(tt.viewer, author.ID, tt.status); got != tt.want {
				t.Fatalf("canReplyToComment() = %v, want %v", got, tt.want)
			}
		})
	}
}
