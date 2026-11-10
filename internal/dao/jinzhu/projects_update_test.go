package jinzhu

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func TestProjectUpdateAuthorizationAndPublicationTransaction(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; PostgreSQL project editing integration test")
	}
	var token [6]byte
	if _, err := rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	prefix := "ut_project_edit_" + hex.EncodeToString(token[:]) + "_"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix, SingularTable: true}, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	models := []any{&dbr.User{}, &dbr.IdentityGroup{}, &dbr.IdentityGroupPermission{}, &dbr.UserIdentityGroup{}, &dbr.Post{}, &dbr.PostContent{}, &dbr.Project{}, &dbr.ProjectImage{}, &dbr.ProjectTag{}, &dbr.ProjectMember{}, &dbr.ProjectFollow{}, &dbr.ReviewTask{}, &dbr.ReviewTaskEvent{}, &dbr.Tag{}, &dbr.AuditLog{}}
	must(db.AutoMigrate(models...))
	defer func() {
		for i := len(models) - 1; i >= 0; i-- {
			db.Migrator().DropTable(models[i])
		}
	}()
	must(db.Exec("CREATE UNIQUE INDEX ON " + prefix + "tag(tag)").Error)
	must(db.Exec("CREATE UNIQUE INDEX ON " + prefix + "project_tag(project_id, name) WHERE is_del=0").Error)
	oldAudit, oldDatabase := conf.AuditSetting, conf.DatabaseSetting
	t.Cleanup(func() { conf.AuditSetting, conf.DatabaseSetting = oldAudit, oldDatabase })
	conf.AuditSetting = &conf.AuditConf{Enabled: true}
	conf.DatabaseSetting = nil
	must(json.Unmarshal([]byte(`{"TablePrefix":"`+prefix+`"}`), &conf.DatabaseSetting))
	guest := &dbr.IdentityGroup{Key: "guest", Name: "Guest"}
	must(db.Create(guest).Error)
	for _, permission := range []string{authz.PostCreate, authz.ContentUpload} {
		must(db.Create(&dbr.IdentityGroupPermission{GroupID: guest.ID, Permission: permission}).Error)
	}
	newUser := func(name string, operator bool) *ms.User {
		t.Helper()
		u := &ms.User{Model: &ms.Model{}, Username: name, Nickname: name, Status: ms.UserStatusNormal, IsOperator: operator}
		must(db.Create(u).Error)
		must(dbr.LoadUserIdentities(db, u))
		return u
	}
	owner, stranger, reviewer := newUser("owner", false), newUser("stranger", false), newUser("reviewer", true)
	post := &ms.Post{UserID: owner.ID, Visibility: ms.PostVisitPublic, AuditStatus: ms.PostAuditApproved, Tags: "项目"}
	must(db.Create(post).Error)
	project := &ms.Project{CreatorID: owner.ID, PostID: post.ID, Title: "Original", Intro: "Original intro", URL: "https://original.example", FollowCount: 1}
	must(db.Create(project).Error)
	must(db.Create(&dbr.ProjectImage{ProjectID: project.ID, URL: "https://images.example/old.png"}).Error)
	must(db.Create(&dbr.ProjectTag{ProjectID: project.ID, Name: "Old"}).Error)
	must(db.Create(&dbr.ProjectMember{ProjectID: project.ID, UserID: owner.ID, Role: "owner"}).Error)
	must(db.Create(&dbr.ProjectMember{ProjectID: project.ID, UserID: stranger.ID, Role: "member"}).Error)
	must(db.Create(&dbr.ProjectFollow{ProjectID: project.ID, UserID: owner.ID}).Error)
	must(db.Create(&dbr.PostContent{PostID: post.ID, UserID: owner.ID, Content: "Original", Type: ms.ContentTypeTitle}).Error)
	must(db.Create(&dbr.PostContent{PostID: post.ID, UserID: owner.ID, Content: "https://images.example/old.png", Type: ms.ContentTypeImage}).Error)
	must(db.Create(&dbr.PostContent{PostID: post.ID, UserID: owner.ID, Content: "project:1", Type: ms.ContentTypeLink}).Error)
	must(db.Create(&dbr.Tag{UserID: owner.ID, Tag: "项目", QuoteNum: 1}).Error)
	service := &projectSrv{db: db}
	persisted := 0
	input := func(title string) *core.ProjectUpdateInput {
		return &core.ProjectUpdateInput{
			Project: &ms.Project{Model: &ms.Model{ID: project.ID}, Title: title, Intro: "Updated intro", URL: "https://new.example"},
			Images:  []*ms.ProjectImage{{URL: "https://images.example/new.png", Sort: 0}}, Tags: []*ms.ProjectTag{{Name: "Go"}},
			PersistMedia: func() error { persisted++; return nil },
		}
	}
	assertTitle := func(want string) {
		t.Helper()
		var row ms.Project
		must(db.First(&row, project.ID).Error)
		if row.Title != want || row.FollowCount != 1 || row.CreatorID != owner.ID || row.PostID != post.ID {
			t.Fatalf("stored project corrupted: %+v", row)
		}
	}
	assertTopicCount := func(want int64) {
		t.Helper()
		var row dbr.Tag
		must(db.Where("tag=?", "项目").First(&row).Error)
		if row.QuoteNum != want {
			t.Fatalf("topic count=%d, want %d", row.QuoteNum, want)
		}
	}
	taskFor := func() ms.ReviewTask {
		t.Helper()
		var task ms.ReviewTask
		must(db.Where("kind=? AND target_id=?", ms.ReviewPost, post.ID).First(&task).Error)
		return task
	}
	assertHidden := func() {
		t.Helper()
		if _, err := service.GetProject(nil, project.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("unreviewed/private project visible to guest: %v", err)
		}
	}
	for _, nonOwner := range []*ms.User{stranger, reviewer} {
		if _, err := service.UpdateProject(nonOwner, input("Hijacked")); !errors.Is(err, authz.ErrDenied) || persisted != 0 {
			t.Fatalf("non-owner edit/persistence allowed: %v persisted=%d", err, persisted)
		}
	}
	assertTitle("Original")
	for _, permission := range []string{authz.PostCreate, authz.ContentUpload} {
		must(db.Where("group_id=? AND permission=?", guest.ID, permission).Delete(&dbr.IdentityGroupPermission{}).Error)
		if _, err := service.UpdateProject(owner, input("Stale grant")); !errors.Is(err, authz.ErrDenied) || persisted != 0 {
			t.Fatalf("revoked %s allowed using stale actor: %v persisted=%d", permission, err, persisted)
		}
		must(db.Create(&dbr.IdentityGroupPermission{GroupID: guest.ID, Permission: permission}).Error)
	}
	for _, model := range []any{&ms.Post{}, &ms.Project{}} {
		id := post.ID
		if _, ok := model.(*ms.Project); ok {
			id = project.ID
		}
		must(db.Model(model).Where("id=?", id).Update("is_del", 1).Error)
		if _, err := service.UpdateProject(owner, input("Deleted")); !errors.Is(err, gorm.ErrRecordNotFound) || persisted != 0 {
			t.Fatalf("deleted target allowed: %v persisted=%d", err, persisted)
		}
		must(db.Unscoped().Model(model).Where("id=?", id).Update("is_del", 0).Error)
	}
	failed := input("Should roll back")
	failed.PersistMedia = func() error { return errors.New("storage offline") }
	if result, err := service.UpdateProject(owner, failed); err == nil || result != nil || failed.Post != nil {
		t.Fatalf("storage failure committed: result=%+v err=%v", result, err)
	}
	assertTitle("Original")
	failed = input("Should roll back")
	failed.Tags = []*ms.ProjectTag{{Name: "duplicate"}, {Name: "duplicate"}}
	if result, err := service.UpdateProject(owner, failed); err == nil || result != nil || failed.Post != nil {
		t.Fatalf("tag failure committed: result=%+v err=%v", result, err)
	}
	assertTitle("Original")
	var oldImage dbr.ProjectImage
	must(db.Where("project_id=?", project.ID).First(&oldImage).Error)
	if oldImage.URL != "https://images.example/old.png" {
		t.Fatal("tag failure did not roll back image replacement")
	}
	// Failure late in review assignment rolls back metadata, preview, topic
	// accounting, images, tags and audit status together.
	must(db.Exec("ALTER TABLE " + prefix + "review_task_event ADD CONSTRAINT reject_submission CHECK (event <> 'submitted')").Error)
	failed = input("Review queue failure")
	if result, err := service.UpdateProject(owner, failed); err == nil || result != nil || failed.Post != nil {
		t.Fatalf("review queue failure committed: result=%+v err=%v", result, err)
	}
	must(db.Exec("ALTER TABLE " + prefix + "review_task_event DROP CONSTRAINT reject_submission").Error)
	assertTitle("Original")
	assertTopicCount(1)
	var unchangedPost ms.Post
	must(db.First(&unchangedPost, post.ID).Error)
	if unchangedPost.AuditStatus != ms.PostAuditApproved || unchangedPost.Visibility != ms.PostVisitPublic {
		t.Fatalf("review queue failure changed publication: %+v", unchangedPost)
	}
	change := input("Edited")
	updated, err := service.UpdateProject(owner, change)
	must(err)
	if owner.HasPermission(authz.PostView) || updated.Title != "Edited" || !updated.Followed || updated.FollowCount != 1 || len(updated.Members) != 2 || len(updated.Tags) != 1 || updated.Tags[0] != "Go" || change.Post == nil || change.Post.AuditStatus != ms.PostAuditPending || change.Post.Visibility != ms.PostVisitPublic {
		t.Fatalf("incorrect update response/publication: %+v post=%+v", updated, change.Post)
	}
	assertHidden()
	assertTopicCount(0)
	var preview []ms.PostContent
	must(db.Where("post_id=?", post.ID).Order("sort,id").Find(&preview).Error)
	if len(preview) != 3 {
		t.Fatalf("preview/link contents lost: %+v", preview)
	}
	for _, item := range preview {
		if item.Type == ms.ContentTypeTitle && item.Content != "Edited" || item.Type == ms.ContentTypeImage && item.Content != "https://images.example/new.png" {
			t.Fatalf("preview not synchronized: %+v", item)
		}
	}
	firstTask := taskFor()
	// Only intro changes: title and preview remain the same, but old review UI
	// must still be unable to approve the newly edited project.
	change = input("Edited")
	change.Project.Intro = "Second intro revision"
	_, err = service.UpdateProject(owner, change)
	must(err)
	secondTask := taskFor()
	if secondTask.Revision <= firstTask.Revision || secondTask.State != ms.ReviewPending {
		t.Fatalf("pending edit did not invalidate review: before=%+v after=%+v", firstTask, secondTask)
	}
	if _, err := (&reviewSrv{db: db}).DecideReview(reviewer, firstTask.ID, firstTask.Revision, "approve", ""); !errors.Is(err, core.ErrReviewStale) {
		t.Fatalf("stale review approved changed project: %v", err)
	}
	assertTopicCount(0)
	_, err = (&reviewSrv{db: db}).DecideReview(reviewer, secondTask.ID, secondTask.Revision, "approve", "")
	must(err)
	assertTopicCount(1)
	if _, err := service.GetProject(nil, project.ID); err != nil {
		t.Fatalf("approved project not public: %v", err)
	}
	// Fresh publish exemption preserves an approved publication, even though
	// the actor snapshot does not contain the new permission.
	must(db.Create(&dbr.IdentityGroupPermission{GroupID: guest.ID, Permission: authz.PublishUnreviewed}).Error)
	change = input("Exempt edit")
	_, err = service.UpdateProject(owner, change)
	must(err)
	if change.Post.AuditStatus != ms.PostAuditApproved || change.Post.Visibility != ms.PostVisitPublic {
		t.Fatalf("exempt edit unpublished project: %+v", change.Post)
	}
	assertTopicCount(1)
	// Editing must never republish an existing rejected/private project.
	must(db.Model(&ms.Post{}).Where("id=?", post.ID).Updates(map[string]any{"audit_status": ms.PostAuditRejected, "visibility": ms.PostVisitPrivate}).Error)
	change = input("Private edit")
	_, err = service.UpdateProject(owner, change)
	must(err)
	if change.Post.AuditStatus != ms.PostAuditRejected || change.Post.Visibility != ms.PostVisitPrivate {
		t.Fatalf("edit republished rejected private project: %+v", change.Post)
	}
	assertHidden()
}
