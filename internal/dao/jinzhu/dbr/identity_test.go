package dbr

import (
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
)

func TestUserPermissionsFailClosed(t *testing.T) {
	tests := []struct {
		name       string
		user       *User
		permission string
		want       bool
	}{
		{"missing user", nil, authz.PostView, false},
		{"legacy admin grants nothing", &User{Status: UserStatusNormal, IsAdmin: true, Roles: "admin,operator"}, authz.UserManage, false},
		{"explicit grant", &User{Status: UserStatusNormal, Permissions: []string{authz.PostView}}, authz.PostView, true},
		{"missing grant", &User{Status: UserStatusNormal, Permissions: []string{authz.PostView}}, authz.UserManage, false},
		{"operator", &User{Status: UserStatusNormal, IsOperator: true}, authz.UserManage, true},
		{"unknown operator permission", &User{Status: UserStatusNormal, IsOperator: true}, "made.up", false},
		{"banned operator", &User{Status: UserStatusClosed, IsOperator: true}, authz.PostView, false},
		{"deleted operator", &User{Model: &Model{IsDel: 1}, Status: UserStatusNormal, IsOperator: true}, authz.PostView, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.user.HasPermission(test.permission); got != test.want {
				t.Fatalf("HasPermission() = %v, want %v", got, test.want)
			}
		})
	}
	u := &User{Status: UserStatusNormal, Permissions: []string{authz.PostView, authz.PostView, "unknown"}}
	if got := u.PermissionList(); len(got) != 1 || got[0] != authz.PostView {
		t.Fatalf("effective permissions: %v", got)
	}
	u.IsOperator = true
	u.Groups = []IdentityGroup{{Key: "admin"}}
	if len(u.GroupList()) != 0 {
		t.Fatal("operator must have no identity membership")
	}
	if len(u.PermissionList()) != len(authz.All()) {
		t.Fatal("operator must resolve the known catalog")
	}
}
