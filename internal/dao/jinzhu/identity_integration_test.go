//go:build migration

package jinzhu

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"github.com/BZYA-Community/WebsiteCore/internal/testutil"
	"gorm.io/gorm"
)

func createIdentityUser(t *testing.T, db *gorm.DB, name, kind string) *ms.User {
	t.Helper()
	u := &ms.User{Model: &dbr.Model{}, Username: name, Status: ms.UserStatusNormal, Phone: "test-bound", AccountType: "member", MemberIdentity: ms.Identity(kind)}
	if kind == "admin" || kind == "operator" {
		u.AccountType, u.Roles, u.MemberIdentity = kind, kind, nil
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	return u
}

func requireAccessError(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestIdentityAccountAndCourseLifecycle(t *testing.T) {
	db, _ := testutil.Postgres(t, 25)
	users, courses := &userManageSrv{db: db}, &courseManageSrv{db: db}
	op := createIdentityUser(t, db, "operator", "operator")
	admin := createIdentityUser(t, db, "admin", "admin")
	student := createIdentityUser(t, db, "student", "student")
	teacher := createIdentityUser(t, db, "teacher", "teacher")
	other := createIdentityUser(t, db, "other", "teacher")
	mentor := createIdentityUser(t, db, "mentor", "student")
	_, err := users.CreateAdmin(admin.ID, &ms.User{Username: "forbidden"})
	requireAccessError(t, err, dbr.ErrPermission)
	managed, err := users.CreateAdmin(op.ID, &ms.User{Model: &dbr.Model{}, Username: "managed"})
	if err != nil {
		t.Fatal(err)
	}
	if !managed.MustChangePassword || managed.MemberIdentity != nil || managed.Roles != ms.RoleAdmin {
		t.Fatal("admin not dedicated or temporary password not enforced")
	}
	requireAccessError(t, users.ChangeAccountStatus(admin.ID, managed.ID, ms.UserStatusClosed), nil)
	requireAccessError(t, users.ChangeAccountStatus(admin.ID, managed.ID, ms.UserStatusNormal), nil)
	requireAccessError(t, users.ChangeAccountStatus(admin.ID, admin.ID, ms.UserStatusClosed), dbr.ErrPermission)
	requireAccessError(t, users.RemoveAdminRole(admin.ID, admin.ID), dbr.ErrPermission)
	requireAccessError(t, users.ChangeAccountStatus(admin.ID, op.ID, ms.UserStatusClosed), dbr.ErrPermission)
	requireAccessError(t, users.RemoveAdminRole(admin.ID, op.ID), dbr.ErrPermission)
	requireAccessError(t, users.ChangeAccountStatus(student.ID, managed.ID, ms.UserStatusClosed), dbr.ErrPermission)
	requireAccessError(t, users.RemoveAdminRole(admin.ID, managed.ID), nil)
	if err := db.First(managed, managed.ID).Error; err != nil {
		t.Fatal(err)
	}
	if managed.Status != ms.UserStatusClosed || managed.Roles != "" || managed.MemberIdentity != nil {
		t.Fatal("removing admin converted it into member")
	}
	requireAccessError(t, users.ChangeAccountStatus(op.ID, managed.ID, ms.UserStatusNormal), nil)
	requireAccessError(t, users.ChangeMemberAccess(op.ID, managed.ID, ms.MemberTeacher, false, false), dbr.ErrPermission)
	requireAccessError(t, users.ChangeMemberAccess(admin.ID, mentor.ID, ms.MemberStudent, true, false), nil)
	requireAccessError(t, users.ChangeMemberAccess(admin.ID, teacher.ID, ms.MemberTeacher, true, true), nil)
	// Mentor is independent: removing Teacher does not require removing Mentor.
	requireAccessError(t, users.ChangeMemberAccess(admin.ID, teacher.ID, ms.MemberStudent, true, true), nil)
	if err := db.First(teacher, teacher.ID).Error; err != nil || teacher.IsTeacher() || !teacher.IsMentor {
		t.Fatal("independent Mentor was not retained", err)
	}
	requireAccessError(t, users.ChangeMemberAccess(admin.ID, teacher.ID, ms.MemberTeacher, false, true), nil)
	course, err := courses.CreateCourseAs(teacher.ID, &ms.Course{Model: &dbr.Model{}, Title: "Owned", TeacherID: other.ID})
	if err != nil {
		t.Fatal(err)
	}
	if course.TeacherID != teacher.ID {
		t.Fatal("client controlled course owner")
	}
	for _, actor := range []*ms.User{student, mentor} {
		_, err = courses.CreateCourseAs(actor.ID, &ms.Course{Model: &dbr.Model{}, Title: "Forbidden"})
		requireAccessError(t, err, dbr.ErrPermission)
	}
	for _, actor := range []*ms.User{admin, op} {
		owned, err := courses.CreateCourseAs(actor.ID, &ms.Course{Model: &dbr.Model{}, Title: "Staff course", TeacherID: teacher.ID})
		requireAccessError(t, err, nil)
		if owned.TeacherID != actor.ID {
			t.Fatal("staff course was not assigned to its creator")
		}
		requireAccessError(t, courses.DeleteCourseAs(actor.ID, owned), nil)
	}
	requireAccessError(t, courses.UpdateCourseAs(other.ID, course), dbr.ErrPermission)
	requireAccessError(t, courses.UpdateCourseAs(teacher.ID, course), nil)
	requireAccessError(t, courses.DeleteCourseAs(teacher.ID, course), dbr.ErrPermission)
	requireAccessError(t, users.ChangeMemberAccess(admin.ID, teacher.ID, ms.MemberStudent, false, false), dbr.ErrTeacherInUse)
	requireAccessError(t, users.DeleteMember(admin.ID, teacher.ID), dbr.ErrTeacherInUse)
	requireAccessError(t, users.ChangeAccountStatus(admin.ID, teacher.ID, ms.UserStatusClosed), nil)
	requireAccessError(t, courses.UpdateCourseAs(teacher.ID, course), dbr.ErrPermission)
	requireAccessError(t, courses.UpdateCourseAs(admin.ID, course), nil)
	var visible int64
	if err := db.Model(&ms.Course{}).Where("id = ?", course.ID).Count(&visible).Error; err != nil || visible != 1 {
		t.Fatal("banning hid course", err)
	}
	for _, target := range []*ms.User{student, mentor} {
		course.TeacherID = target.ID
		requireAccessError(t, courses.UpdateCourseAs(admin.ID, course), dbr.ErrPermission)
	}
	for _, target := range []*ms.User{admin, op} {
		course.TeacherID = target.ID
		requireAccessError(t, courses.UpdateCourseAs(admin.ID, course), nil)
	}
	course.TeacherID = other.ID
	requireAccessError(t, courses.UpdateCourseAs(admin.ID, course), nil)
	course.TeacherID = teacher.ID // banned Teacher cannot receive a transfer
	requireAccessError(t, courses.UpdateCourseAs(admin.ID, course), dbr.ErrPermission)
	requireAccessError(t, users.DeleteMember(admin.ID, teacher.ID), nil)
	requireAccessError(t, courses.DeleteCourseAs(admin.ID, course), nil)
	requireAccessError(t, users.DeleteMember(admin.ID, mentor.ID), nil)
}

func TestIndependentMentorChangesCancelOnlyInvalidRequests(t *testing.T) {
	db, _ := testutil.Postgres(t, 25)
	users, messages := &userManageSrv{db: db}, &messageSrv{db: db}
	admin := createIdentityUser(t, db, "admin", "admin")
	student := createIdentityUser(t, db, "student", "student")
	mentor := createIdentityUser(t, db, "mentor", "student")
	requireAccessError(t, users.ChangeMemberAccess(admin.ID, mentor.ID, ms.MemberStudent, true, false), nil)
	send := func(from, to int64) error {
		_, err := messages.SendWhisper(&ms.Message{Model: &dbr.Model{}, SenderUserID: from, ReceiverUserID: to, Content: "contact"})
		return err
	}
	requireAccessError(t, send(mentor.ID, student.ID), nil)
	// Adding Teacher keeps the Mentor request valid.
	requireAccessError(t, users.ChangeMemberAccess(admin.ID, mentor.ID, ms.MemberTeacher, true, false), nil)
	c, err := getConversation(db, mentor.ID, student.ID, false)
	if err != nil || c.PendingSenderID == nil || *c.PendingSenderID != mentor.ID {
		t.Fatalf("valid request lost on adding Teacher: %+v %v", c, err)
	}
	// Removing Mentor cancels its proactive contact; Teacher cannot re-initiate it.
	requireAccessError(t, users.ChangeMemberAccess(admin.ID, mentor.ID, ms.MemberTeacher, false, false), nil)
	c, err = getConversation(db, mentor.ID, student.ID, false)
	if err != nil || c.PendingSenderID != nil {
		t.Fatalf("invalid Mentor request survived: %+v %v", c, err)
	}
	requireAccessError(t, send(mentor.ID, student.ID), dbr.ErrWhisperIdentity)
	requireAccessError(t, send(student.ID, mentor.ID), nil)
	// Removing Teacher while retaining Mentor keeps an incoming request valid.
	requireAccessError(t, users.ChangeMemberAccess(admin.ID, mentor.ID, ms.MemberStudent, true, false), nil)
	requireAccessError(t, send(mentor.ID, student.ID), nil)
	requireAccessError(t, send(student.ID, mentor.ID), nil)
	requireAccessError(t, users.ChangeMemberAccess(admin.ID, mentor.ID, ms.MemberStudent, false, false), nil)
	requireAccessError(t, send(student.ID, mentor.ID), dbr.ErrWhisperIdentity)
}

func TestWhisperPersistenceAndConcurrentFirstMessage(t *testing.T) {
	db, _ := testutil.Postgres(t, 25)
	users, messages := &userManageSrv{db: db}, &messageSrv{db: db}
	admin := createIdentityUser(t, db, "admin", "admin")
	a := createIdentityUser(t, db, "student", "student")
	b := createIdentityUser(t, db, "teacher", "teacher")
	send := func(from, to int64, body string) error {
		_, err := messages.SendWhisper(&ms.Message{Model: &dbr.Model{}, SenderUserID: from, ReceiverUserID: to, Content: body})
		return err
	}
	start, result := make(chan struct{}), make(chan error, 16)
	for i := 0; i < cap(result); i++ {
		go func(i int) { <-start; result <- send(a.ID, b.ID, fmt.Sprint(i)) }(i)
	}
	close(start)
	success := 0
	for i := 0; i < cap(result); i++ {
		if err := <-result; err == nil {
			success++
		} else {
			requireAccessError(t, err, dbr.ErrWhisperPending)
		}
	}
	if success != 1 {
		t.Fatalf("%d concurrent first messages committed", success)
	}
	var count int64
	if err := db.Model(&ms.Message{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("message count %d: %v", count, err)
	}
	requireAccessError(t, users.ChangeMemberAccess(admin.ID, b.ID, ms.MemberStudent, false, false), nil)
	requireAccessError(t, send(a.ID, b.ID, "invalid"), dbr.ErrWhisperIdentity)
	requireAccessError(t, users.ChangeMemberAccess(admin.ID, b.ID, ms.MemberTeacher, false, false), nil)
	requireAccessError(t, send(a.ID, b.ID, "new request"), nil)
	requireAccessError(t, messages.SetWhisperBlock(b.ID, a.ID, true), nil)
	requireAccessError(t, send(a.ID, b.ID, "blocked"), dbr.ErrWhisperBlocked)
	requireAccessError(t, send(b.ID, a.ID, "blocked reply"), dbr.ErrWhisperBlocked)
	requireAccessError(t, messages.SetWhisperBlock(b.ID, a.ID, false), nil)
	requireAccessError(t, send(a.ID, b.ID, "request after unblock"), nil)
	requireAccessError(t, send(b.ID, a.ID, "reply"), nil)
	requireAccessError(t, messages.SetWhisperBlock(a.ID, b.ID, true), nil)
	requireAccessError(t, messages.SetWhisperBlock(a.ID, b.ID, false), nil)
	requireAccessError(t, send(a.ID, b.ID, "established 1"), nil)
	requireAccessError(t, send(a.ID, b.ID, "established 2"), nil)
	requireAccessError(t, users.ChangeMemberAccess(admin.ID, b.ID, ms.MemberStudent, false, false), nil)
	requireAccessError(t, send(b.ID, a.ID, "readonly"), dbr.ErrWhisperIdentity)
	requireAccessError(t, users.ChangeMemberAccess(admin.ID, b.ID, ms.MemberTeacher, false, false), nil)
	requireAccessError(t, send(a.ID, b.ID, "restored 1"), nil)
	requireAccessError(t, send(a.ID, b.ID, "restored 2"), nil)
	if err := db.Model(b).Update("phone", "").Error; err != nil {
		t.Fatal(err)
	}
	requireAccessError(t, send(a.ID, b.ID, "no receiver phone"), nil)
	requireAccessError(t, send(b.ID, a.ID, "no sender phone"), dbr.ErrWhisperPhone)
	requireAccessError(t, messages.SetWhisperBlock(a.ID, admin.ID, true), dbr.ErrPermission)
}

func TestSimultaneousIdentityChangesCancelPending(t *testing.T) {
	db, _ := testutil.Postgres(t, 25)
	users, messages := &userManageSrv{db: db}, &messageSrv{db: db}
	adminA := createIdentityUser(t, db, "admina", "admin")
	adminB := createIdentityUser(t, db, "adminb", "admin")
	a := createIdentityUser(t, db, "teachera", "teacher")
	b := createIdentityUser(t, db, "teacherb", "teacher")
	if _, err := messages.SendWhisper(&ms.Message{Model: &dbr.Model{}, SenderUserID: a.ID, ReceiverUserID: b.ID}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, ids := range [][2]int64{{adminA.ID, a.ID}, {adminB.ID, b.ID}} {
		wg.Add(1)
		go func(ids [2]int64) {
			defer wg.Done()
			errs <- users.ChangeMemberAccess(ids[0], ids[1], ms.MemberStudent, false, false)
		}(ids)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		requireAccessError(t, err, nil)
	}
	c, err := getConversation(db, a.ID, b.ID, false)
	if err != nil || c.PendingSenderID != nil {
		t.Fatalf("invalid pending survived simultaneous changes: %+v %v", c, err)
	}
}

func TestStaleProfileCannotRestoreCredentialsOrPrivileges(t *testing.T) {
	db, _ := testutil.Postgres(t, 25)
	users := &userManageSrv{db: db}
	admin := createIdentityUser(t, db, "admin", "admin")
	student := createIdentityUser(t, db, "student", "student")
	stale := *student
	requireAccessError(t, users.ChangeMemberAccess(admin.ID, student.ID, ms.MemberTeacher, true, true), nil)
	if err := db.Model(student).Updates(map[string]any{"password": "new-hash", "salt": "new-salt"}).Error; err != nil {
		t.Fatal(err)
	}
	stale.Nickname = "new nickname"
	requireAccessError(t, stale.Update(db, "nickname"), nil)
	requireAccessError(t, stale.Update(db, "roles"), dbr.ErrPermission)
	if err := db.First(student, student.ID).Error; err != nil {
		t.Fatal(err)
	}
	if student.Password != "new-hash" || student.Salt != "new-salt" || !student.IsTeacher() || !student.IsMentor || student.Roles != ms.RoleAuditor {
		t.Fatal("stale profile write restored credentials or identity")
	}
}

func TestCourseTransferRacesWithTeacherDemotion(t *testing.T) {
	db, _ := testutil.Postgres(t, 25)
	users, courses := &userManageSrv{db: db}, &courseManageSrv{db: db}
	adminA := createIdentityUser(t, db, "admina", "admin")
	adminB := createIdentityUser(t, db, "adminb", "admin")
	owner := createIdentityUser(t, db, "owner", "teacher")
	target := createIdentityUser(t, db, "target", "teacher")
	course, err := courses.CreateCourseAs(owner.ID, &ms.Course{Model: &dbr.Model{}, Title: "Race"})
	requireAccessError(t, err, nil)
	course.TeacherID = target.ID
	start, errs := make(chan struct{}), make(chan error, 2)
	go func() { <-start; errs <- courses.UpdateCourseAs(adminA.ID, course) }()
	go func() {
		<-start
		errs <- users.ChangeMemberAccess(adminB.ID, target.ID, ms.MemberStudent, false, false)
	}()
	close(start)
	successes := 0
	for range 2 {
		err := <-errs
		if err == nil {
			successes++
		} else if !errors.Is(err, dbr.ErrPermission) && !errors.Is(err, dbr.ErrTeacherInUse) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one mutation, got %d", successes)
	}
	if err := db.First(course, course.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(target, target.ID).Error; err != nil {
		t.Fatal(err)
	}
	if course.TeacherID == target.ID && !target.IsTeacher() {
		t.Fatal("course assigned to demoted teacher")
	}
}
