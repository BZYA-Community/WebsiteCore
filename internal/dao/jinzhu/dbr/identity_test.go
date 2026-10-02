package dbr

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func identityUser(id int64, kind string) *User {
	u := &User{Model: &Model{ID: id}, Status: UserStatusNormal, Phone: "test-bound", AccountType: "member"}
	identity := MemberStudent
	switch kind {
	case "teacher", "teacher-mentor", "teacher-auditor":
		identity = MemberTeacher
	case "admin", "operator":
		u.AccountType, u.Roles = kind, kind
		return u
	}
	u.MemberIdentity = &identity
	u.IsMentor = kind == "mentor" || kind == "teacher-mentor"
	if strings.HasSuffix(kind, "auditor") {
		u.Roles = RoleAuditor
	}
	return u
}

func TestIdentityCapabilitiesAndPublicProjection(t *testing.T) {
	for _, kind := range []string{"student", "student-auditor", "teacher", "mentor", "teacher-mentor", "teacher-auditor", "admin", "operator"} {
		t.Run(kind, func(t *testing.T) {
			u := identityUser(1, kind)
			if !u.ValidIdentity() {
				t.Fatal("valid identity rejected")
			}
			management := kind == "admin" || kind == "operator"
			teacher := kind == "teacher" || kind == "teacher-mentor" || kind == "teacher-auditor"
			if u.CanPublishDirectly() != management || u.CanCreateCourse() != (management || teacher) || u.CanManageUsers() != management {
				t.Fatal("unrelated roles granted publication or course capabilities")
			}
			if u.CanEditCourse(1) != (management || teacher) || u.CanEditCourse(2) != management || u.CanDeleteCourse() != management {
				t.Fatal("course ownership not enforced")
			}
			if u.CanCreateAdmin() != (kind == "operator") || u.CanManageAdmins() != management || u.CanAudit() != (management || strings.HasSuffix(kind, "auditor")) {
				t.Fatal("inherited management capabilities incorrect")
			}
			if u.CanViewSystemInfo() != (kind == "operator") {
				t.Fatal("system information must be restricted to the single Operator")
			}
			if u.CanAuditUser(u.ID) {
				t.Fatal("self audit allowed")
			}
			data, err := json.Marshal(u.Format())
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{"auditor", "is_admin", `"identity"`, "password", "salt"} {
				if strings.Contains(string(data), secret) {
					t.Fatalf("public projection leaks %s", secret)
				}
			}
			u.Status = UserStatusClosed
			if u.CanAudit() || u.CanPublishDirectly() || u.CanManageUsers() || u.CanCreateCourse() || u.CanEditCourse(1) || u.CanViewSystemInfo() || u.CanCreateAdmin() || u.CanManageAdmins() {
				t.Fatal("banned user has capabilities")
			}
		})
	}
}

func TestWhisperFirstContactMatrix(t *testing.T) {
	kinds := []string{"student", "student-auditor", "teacher", "mentor", "teacher-mentor", "teacher-auditor", "admin", "operator"}
	for _, from := range kinds {
		for _, to := range kinds {
			t.Run(from+"/"+to, func(t *testing.T) {
				sender, receiver := identityUser(1, from), identityUser(2, to)
				c := &WhisperConversation{LowUserID: 1, HighUserID: 2}
				allowed := from == "admin" || from == "operator" || from == "mentor" || from == "teacher-mentor" ||
					to == "teacher" || to == "mentor" || to == "teacher-mentor" || to == "teacher-auditor"
				if err := c.CanSend(sender, receiver); (err == nil) != allowed {
					t.Fatalf("first message: %v", err)
				}
				if !allowed {
					return
				}
				// A receiver without a phone may still receive; every sender needs one.
				receiver.Phone = ""
				if err := c.CanSend(sender, receiver); err != nil {
					t.Fatal(err)
				}
				c.Sent(sender)
				if err := c.CanSend(sender, receiver); (err == nil) != sender.IsAdminLevel() {
					t.Fatalf("pending limit: %v", err)
				}
				if err := c.CanSend(receiver, sender); !errors.Is(err, ErrWhisperPhone) {
					t.Fatalf("phone gate: %v", err)
				}
				receiver.Phone = "test-bound"
				if err := c.CanSend(receiver, sender); err != nil {
					t.Fatalf("reply: %v", err)
				}
				c.Sent(receiver)
				if !c.Established || c.PendingSenderID != nil {
					t.Fatal("reply did not establish conversation")
				}
				if err := c.CanSend(sender, receiver); err != nil {
					t.Fatalf("established send: %v", err)
				}
			})
		}
	}
}

func TestWhisperRechecksIdentityAndBlocks(t *testing.T) {
	a, b := identityUser(1, "teacher"), identityUser(2, "student")
	c := &WhisperConversation{LowUserID: 1, HighUserID: 2, Established: true}
	c.BlockedByHigh = true
	if err := c.CanSend(a, b); !errors.Is(err, ErrWhisperBlocked) {
		t.Fatal(err)
	}
	if err := c.CanSend(b, a); !errors.Is(err, ErrWhisperBlocked) {
		t.Fatal(err)
	}
	c.BlockedByHigh = false
	*a.MemberIdentity = MemberStudent
	if err := c.CanSend(a, b); !errors.Is(err, ErrWhisperIdentity) {
		t.Fatal("established history bypassed current identity")
	}
	*a.MemberIdentity = MemberTeacher
	if err := c.CanSend(a, b); err != nil {
		t.Fatal("restored identity cannot use established conversation")
	}
	a.Phone = ""
	if err := c.CanSend(a, b); !errors.Is(err, ErrWhisperPhone) {
		t.Fatal(err)
	}
	admin := identityUser(1, "admin")
	c.BlockedByHigh, b.Status = true, UserStatusClosed
	if err := c.CanSend(admin, b); err != nil {
		t.Fatalf("administrative contact denied: %v", err)
	}
}
