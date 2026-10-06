package jinzhu

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestIdentityGroupValidation(t *testing.T) {
	for _, key := range []string{"", "operator", "UPPER", "a/b"} {
		if err := normalizeIdentityGroup(&ms.IdentityGroup{Key: key, Name: "Group"}); !errors.Is(err, authz.ErrInvalid) {
			t.Fatalf("accepted key %q", key)
		}
	}
	group := &ms.IdentityGroup{Key: "reviewer", Name: " Reviewer ", Permissions: []string{authz.PostView, authz.PostView}}
	if err := normalizeIdentityGroup(group); err != nil || group.Name != "Reviewer" || len(group.Permissions) != 1 {
		t.Fatalf("normalize = %+v, %v", group, err)
	}
	group.Permissions = []string{"unknown"}
	if !errors.Is(normalizeIdentityGroup(group), authz.ErrInvalid) {
		t.Fatal("accepted unknown permission")
	}
}

func TestIdentityPolicyTransactions(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; PostgreSQL identity integration test")
	}
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	prefix := "ut_identity_" + hex.EncodeToString(buf) + "_"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix, SingularTable: true}})
	if err != nil {
		t.Fatal(err)
	}
	models := []any{&dbr.User{}, &dbr.IdentityGroup{}, &dbr.IdentityGroupPermission{}, &dbr.UserIdentityGroup{}, &dbr.IdentityOperationLog{}}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for i := len(models) - 1; i >= 0; i-- {
			if err := db.Migrator().DropTable(models[i]); err != nil {
				t.Log(err)
			}
		}
	})
	s := &identitySrv{db: db}
	seedGroup := func(key string, permissions ...string) *ms.IdentityGroup {
		t.Helper()
		group := &ms.IdentityGroup{Key: key, Name: key, Builtin: true}
		if err := db.Create(group).Error; err != nil {
			t.Fatal(err)
		}
		for _, permission := range permissions {
			if err := db.Create(&dbr.IdentityGroupPermission{GroupID: group.ID, Permission: permission}).Error; err != nil {
				t.Fatal(err)
			}
		}
		return group
	}
	guest := seedGroup("guest", authz.PostView, authz.CourseCatalog, authz.ProfileEdit)
	seedGroup("member", authz.PostView, authz.CourseCatalog, authz.CourseView, authz.PostCreate)
	adminPermissions := slices.DeleteFunc(authz.All(), func(p string) bool { return p == authz.PublishUnreviewed })
	adminGroup := seedGroup("admin", adminPermissions...)
	createUser := func(name, phone string, operator bool) *ms.User {
		t.Helper()
		user := &ms.User{Model: &ms.Model{}, Username: name, Phone: phone, Status: ms.UserStatusNormal, IsOperator: operator}
		if err := db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
		return user
	}
	operator := createUser("operator", "", true)
	admin := createUser("admin", "", false)
	member := createUser("member", "verified", false)
	if err := s.SetUserIdentityGroups(operator, admin.ID, []int64{adminGroup.ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.LoadUserIdentities(admin, member); err != nil {
		t.Fatal(err)
	}
	if admin.HasPermission(authz.PublishUnreviewed) || !admin.HasPermission(authz.UserManage) {
		t.Fatal("admin default boundary")
	}
	if !member.HasPermission(authz.CourseView) {
		t.Fatal("verified member missing view permission")
	}
	anonymous := &ms.User{Status: ms.UserStatusNormal}
	if err := s.LoadUserIdentities(anonymous); err != nil {
		t.Fatal(err)
	}
	if !anonymous.HasPermission(authz.CourseCatalog) || anonymous.HasPermission(authz.CourseView) {
		t.Fatal("anonymous catalog/view boundary")
	}
	group, err := s.SaveIdentityGroup(operator, &ms.IdentityGroup{Key: "publisher", Name: "Publisher", Permissions: []string{authz.PublishUnreviewed}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveIdentityGroup(admin, group); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("admin modified elevated group: %v", err)
	}
	if err := s.SetUserIdentityGroups(admin, admin.ID, []int64{adminGroup.ID}); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("admin changed own memberships: %v", err)
	}
	if err := s.SetUserIdentityGroups(admin, admin.ID, []int64{group.ID}); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("self escalation: %v", err)
	}
	if err := s.SetUserIdentityGroups(operator, operator.ID, []int64{adminGroup.ID}); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("operator membership: %v", err)
	}
	if err := s.SetUserIdentityGroups(operator, member.ID, []int64{guest.ID}); !errors.Is(err, authz.ErrInvalid) {
		t.Fatalf("explicit automatic group: %v", err)
	}
	if err := s.DeleteIdentityGroup(operator, guest.ID); !errors.Is(err, authz.ErrInvalid) {
		t.Fatalf("deleted builtin: %v", err)
	}
	if err := s.SetUserIdentityGroups(operator, member.ID, []int64{group.ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.LoadUserIdentities(member); err != nil {
		t.Fatal(err)
	}
	if !member.HasPermission(authz.PublishUnreviewed) || !member.HasPermission(authz.CourseView) {
		t.Fatal("membership union lost permissions")
	}
	if err := s.DeleteIdentityGroup(operator, group.ID); !errors.Is(err, authz.ErrGroupInUse) {
		t.Fatalf("deleted assigned group: %v", err)
	}

	// An audit failure must roll back both policy and account changes.
	logTable := prefix + "identity_operation_log"
	// Only future writes are checked, keeping prior audit evidence intact.
	if err := db.Exec("ALTER TABLE " + logTable + " ADD CONSTRAINT reject_log CHECK (action NOT IN ('group.save', 'user.groups', 'user.status', 'user.delete')) NOT VALID").Error; err != nil {
		t.Fatal(err)
	}
	group.Name = "Changed"
	if _, err := s.SaveIdentityGroup(operator, group); err == nil {
		t.Fatal("group save succeeded despite failed audit")
	}
	var saved dbr.IdentityGroup
	if err := db.First(&saved, group.ID).Error; err != nil || saved.Name != "Publisher" {
		t.Fatalf("group did not roll back: %+v %v", saved, err)
	}
	if err := s.SetUserIdentityGroups(operator, member.ID, nil); err == nil {
		t.Fatal("membership change succeeded despite failed audit")
	}
	if err := s.LoadUserIdentities(member); err != nil || !member.HasPermission(authz.PublishUnreviewed) {
		t.Fatalf("membership did not roll back: %v", err)
	}
	if err := s.SetManagedUserStatus(operator, member.ID, ms.UserStatusClosed); err == nil {
		t.Fatal("status changed despite failed audit")
	}
	if err := s.DeleteManagedUser(operator, member.ID); err == nil {
		t.Fatal("deleted despite failed audit")
	}
	var unchanged ms.User
	if err := db.First(&unchanged, member.ID).Error; err != nil || unchanged.Status != ms.UserStatusNormal || unchanged.IsDel != 0 {
		t.Fatalf("account did not roll back: %+v %v", unchanged, err)
	}
	if err := db.Exec("ALTER TABLE " + logTable + " DROP CONSTRAINT reject_log").Error; err != nil {
		t.Fatal(err)
	}

	if err := s.SetManagedUserStatus(admin, operator.ID, ms.UserStatusClosed); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("admin banned operator: %v", err)
	}
	if err := s.DeleteManagedUser(operator, operator.ID); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("operator self deletion: %v", err)
	}
	if err := s.SetUserIdentityGroups(operator, admin.ID, nil); err != nil {
		t.Fatal(err)
	}
	// admin still has a stale in-memory grant: service must recheck in the transaction.
	if _, err := s.SaveIdentityGroup(admin, &ms.IdentityGroup{Key: "stale", Name: "Stale"}); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("stale authority accepted: %v", err)
	}
	if err := db.Model(member).Updates(map[string]any{"password": "new credential", "salt": "new salt"}).Error; err != nil {
		t.Fatal(err)
	}
	member.Password, member.Salt = "stale credential", "stale salt"
	member.Nickname = "Updated profile"
	if err := member.Update(db, "nickname"); err != nil {
		t.Fatal(err)
	}
	var current ms.User
	if err := db.First(&current, member.ID).Error; err != nil || current.Password != "new credential" || current.Salt != "new salt" {
		t.Fatalf("profile update replaced current credentials: %v", err)
	}
	if err := member.Update(db, "is_operator"); err == nil {
		t.Fatal("profile updater accepted authority field")
	}
	if err := s.SetManagedUserStatus(operator, member.ID, ms.UserStatusClosed); err != nil {
		t.Fatal(err)
	}
	member.Nickname = "Old request completes"
	member.IsOperator = true
	if err := member.Update(db, "nickname"); err != nil {
		t.Fatal(err)
	}
	loaded, err := (&dbr.User{Model: &dbr.Model{ID: member.ID}}).Get(db)
	if err != nil || loaded.HasPermission(authz.PostView) || loaded.IsOperator {
		t.Fatalf("banned account granted permission: %v", err)
	}
	if err := s.DeleteManagedUser(operator, member.ID); err != nil {
		t.Fatal(err)
	}
	if err := member.Update(db, "nickname"); err != nil {
		t.Fatal(err)
	}
	if err := db.Unscoped().First(&unchanged, member.ID).Error; err != nil || unchanged.IsDel == 0 {
		t.Fatalf("stale profile update restored deleted account: %v", err)
	}
}
