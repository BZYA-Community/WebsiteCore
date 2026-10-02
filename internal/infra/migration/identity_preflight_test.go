//go:build migration

package migration

import (
	"context"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/testutil"
)

func TestIdentityUpgradeEmptyAndPopulated(t *testing.T) {
	for _, deleted := range []int{0, 1} {
		t.Run(map[int]string{0: "active", 1: "soft-deleted"}[deleted], func(t *testing.T) {
			_, db := testutil.Postgres(t, 24)
			if _, err := db.Exec("INSERT INTO p_user (username, is_del) VALUES ('legacy', $1)", deleted); err != nil {
				t.Fatal(err)
			}
			// Reject before the driver creates a version table or marks it dirty.
			if err := CheckIdentityUpgrade(context.Background(), db, "p_"); err == nil {
				t.Fatal("populated database accepted")
			}
			var columns, users int
			if err := db.QueryRow("SELECT COUNT(*) FROM pg_attribute WHERE attrelid='p_user'::regclass AND attname IN ('account_type','member_identity','is_mentor') AND NOT attisdropped").Scan(&columns); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow("SELECT COUNT(*) FROM p_user").Scan(&users); err != nil {
				t.Fatal(err)
			}
			var versionTable bool
			if err := db.QueryRow("SELECT to_regclass('p_schema_migrations') IS NOT NULL").Scan(&versionTable); err != nil {
				t.Fatal(err)
			}
			if columns != 0 || users != 1 || versionTable {
				t.Fatal("preflight changed schema, data or migration state")
			}
		})
	}
	_, db := testutil.Postgres(t, 24)
	if err := CheckIdentityUpgrade(context.Background(), db, "p_"); err != nil {
		t.Fatal(err)
	}
	testutil.Apply(t, db, "0025_explicit_identity.up.sql")
	var oldColumn bool
	if err := db.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid='p_user'::regclass AND attname='is_admin' AND NOT attisdropped)").Scan(&oldColumn); err != nil {
		t.Fatal(err)
	}
	if oldColumn {
		t.Fatal("is_admin still exists")
	}
	testutil.Apply(t, db, "0025_explicit_identity.down.sql")
	testutil.Apply(t, db, "0025_explicit_identity.up.sql")
}

func TestIdentityDatabaseConstraints(t *testing.T) {
	_, db := testutil.Postgres(t, 25)
	for _, tc := range []struct {
		name, kind, role string
		identity         any
		mentor           bool
	}{
		{"member-without-identity", "member", "", nil, false},
		{"admin-with-identity", "admin", "admin", "teacher", false},
		{"admin-mentor", "admin", "admin", nil, true},
		{"dedicated-auditor", "auditor", "auditor", nil, false},
		{"operator-admin", "operator", "operator,admin", nil, false},
		{"admin-auditor", "admin", "admin,auditor", nil, false},
		{"mentor-role", "member", "mentor", "teacher", false},
		{"student-admin", "member", "admin", "student", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := db.Exec("INSERT INTO p_user (username, account_type, roles, member_identity, is_mentor) VALUES ($1,$2,$3,$4,$5)", tc.name, tc.kind, tc.role, tc.identity, tc.mentor)
			if err == nil {
				t.Fatal("invalid identity stored")
			}
		})
	}
	if _, err := db.Exec("INSERT INTO p_user (username) VALUES ('defaultstudent')"); err != nil {
		t.Fatal(err)
	}
	for _, identity := range []string{"student", "teacher"} {
		if _, err := db.Exec("INSERT INTO p_user (username, member_identity, is_mentor) VALUES ($1,$2,true)", identity+"-mentor", identity); err != nil {
			t.Fatalf("independent or overlapping Mentor rejected: %v", err)
		}
	}
	if _, err := db.Exec("INSERT INTO p_user (username, account_type, roles, member_identity) VALUES ('only-operator','operator','operator',NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO p_user (username, account_type, roles, member_identity) VALUES ('second-operator','operator','operator',NULL)"); err == nil {
		t.Fatal("more than one Operator stored")
	}
	var identity string
	if err := db.QueryRow("SELECT member_identity FROM p_user WHERE username='defaultstudent'").Scan(&identity); err != nil || identity != "student" {
		t.Fatalf("default identity %q: %v", identity, err)
	}
	if _, err := db.Exec("UPDATE p_user SET account_type='admin', roles='admin', member_identity=NULL"); err == nil {
		t.Fatal("account conversion accepted")
	}
	if err := CheckIdentityUpgrade(context.Background(), db, "p_"); err != nil {
		t.Fatalf("already-upgraded database rejected: %v", err)
	}
}
