package jinzhu

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

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

func TestColumnSubmissionLimits(t *testing.T) {
	for _, test := range []struct {
		name, kind string
		input      ms.ColumnSubmission
		valid      bool
	}{
		{"article", ms.ColumnArticle, ms.ColumnSubmission{Title: "Article", Body: "# Content"}, true},
		{"collection", ms.ColumnCollection, ms.ColumnSubmission{Title: "Series", Summary: "Introduction", Cover: "cover"}, true},
		{"no article cover", ms.ColumnArticle, ms.ColumnSubmission{Title: "Article", Body: "Text", Cover: "cover"}, false},
		{"empty title", ms.ColumnArticle, ms.ColumnSubmission{Body: "Text"}, false},
		{"empty body", ms.ColumnArticle, ms.ColumnSubmission{Title: "Article", Body: "  "}, false},
		{"limit", ms.ColumnArticle, ms.ColumnSubmission{Title: "Article", Body: strings.Repeat("中", 50000)}, true},
		{"too long", ms.ColumnArticle, ms.ColumnSubmission{Title: "Article", Body: strings.Repeat("中", 50001)}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := normalizeColumnSubmission(test.kind, &test.input); (err == nil) != test.valid {
				t.Fatalf("valid=%v err=%v", test.valid, err)
			}
		})
	}
}

func TestColumnsPublicationCollectionsAndDiscussion(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; PostgreSQL column integration test")
	}
	var token [6]byte
	if _, err := rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	prefix := "ut_columns_" + hex.EncodeToString(token[:]) + "_"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix, SingularTable: true}, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	models := []any{&dbr.User{}, &dbr.IdentityGroup{}, &dbr.IdentityGroupPermission{}, &dbr.UserIdentityGroup{}, &dbr.Column{}, &dbr.ColumnSubmission{}, &dbr.ColumnEntry{}, &dbr.ColumnComment{}, &dbr.ColumnReaction{}, &dbr.Attachment{}, &dbr.ReviewTask{}, &dbr.ReviewTaskEvent{}, &dbr.AuditLog{}, &dbr.Message{}}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for i := len(models) - 1; i >= 0; i-- {
			db.Migrator().DropTable(models[i])
		}
	})
	oldAudit, oldApp := conf.AuditSetting, conf.AppSetting
	conf.AuditSetting = &conf.AuditConf{Enabled: true, DeadlineHours: 48}
	// Preserve the concrete configuration type while supplying the count limit.
	conf.AppSetting = nil
	if err := json.Unmarshal([]byte(`{"MaxCommentCount":1000}`), &conf.AppSetting); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conf.AuditSetting, conf.AppSetting = oldAudit, oldApp })
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	group := func(key string, perms ...string) *dbr.IdentityGroup {
		t.Helper()
		g := &dbr.IdentityGroup{Key: key, Name: key}
		must(db.Create(g).Error)
		for _, p := range perms {
			must(db.Create(&dbr.IdentityGroupPermission{GroupID: g.ID, Permission: p}).Error)
		}
		return g
	}
	group("guest")
	publish := group("publisher", authz.ColumnPublish, authz.CommentCreate, authz.CommunityInteract)
	reviewGroup := group("reviewer", authz.ContentReview, authz.AuditViewAll)
	manageGroup := group("manager", authz.ContentManage, authz.ColumnPublish, authz.IdentityManage)
	exempt := group("exempt", authz.PublishUnreviewed)
	user := func(name string, groups ...*dbr.IdentityGroup) *ms.User {
		t.Helper()
		u := &ms.User{Model: &ms.Model{}, Username: name, Nickname: name, Status: ms.UserStatusNormal}
		must(db.Create(u).Error)
		for _, g := range groups {
			must(db.Create(&dbr.UserIdentityGroup{UserID: u.ID, GroupID: g.ID}).Error)
		}
		must(dbr.LoadUserIdentities(db, u))
		return u
	}
	author := user("author", publish)
	other := user("other", publish)
	reviewer := user("reviewer", reviewGroup)
	manager := user("manager", manageGroup)
	operator := user("operator")
	must(db.Model(operator).Update("is_operator", true).Error)
	operator.IsOperator = true
	s := &columnSrv{db: db}
	review := &reviewSrv{db: db}
	taskFor := func(kind string, id int64) *ms.ReviewTask {
		t.Helper()
		task := &ms.ReviewTask{}
		must(db.Where("kind=? AND target_id=?", kind, id).First(task).Error)
		return task
	}
	decide := func(task *ms.ReviewTask, action string) {
		t.Helper()
		actor := reviewer
		if task.AssigneeID == operator.ID {
			actor = operator
		}
		_, err := review.DecideReview(actor, task.ID, task.Revision, action, "test decision")
		must(err)
	}
	submit := func(u *ms.User, id, expected int64, kind, title, body string) *ms.Column {
		t.Helper()
		row, err := s.SubmitColumn(u, id, expected, kind, &ms.ColumnSubmission{Title: title, Body: body})
		must(err)
		return row
	}
	article := submit(author, 0, 0, ms.ColumnArticle, "Original title", "original searchable body")
	if _, err := s.GetColumn(nil, article.ID, false); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("unreviewed article visible: %v", err)
	}
	task := taskFor(ms.ReviewColumn, article.SubmissionID)
	if _, err := review.DecideReview(author, task.ID, task.Revision, "approve", ""); err == nil {
		t.Fatal("author self-review allowed")
	}
	decide(task, "approve")
	public, err := s.GetColumn(nil, article.ID, false)
	must(err)
	firstPublication := public.PublishedOn
	if public.Title != "Original title" {
		t.Fatalf("wrong public title: %s", public.Title)
	}
	must(s.ColumnAction(manager, article.ID, "invite"))
	update := submit(author, article.ID, article.SubmissionID, ms.ColumnArticle, "New title", "new secret body")
	public, err = s.GetColumn(nil, article.ID, false)
	must(err)
	if public.Title != "Original title" {
		t.Fatal("pending version replaced public content")
	}
	_, total, err := s.ListColumns(nil, core.ColumnQuery{Kind: ms.ColumnArticle, Keyword: "secret", Limit: 20})
	must(err)
	if total != 0 {
		t.Fatal("pending body searchable")
	}
	oldTask := taskFor(ms.ReviewColumn, update.SubmissionID)
	if _, err := s.SubmitColumn(author, article.ID, update.SubmissionID, ms.ColumnArticle, &ms.ColumnSubmission{Title: "Race", Body: "Text"}); !errors.Is(err, core.ErrReviewStale) {
		t.Fatalf("modified a pending submission: %v", err)
	}
	must(s.ColumnAction(author, article.ID, "withdraw"))
	if _, err := review.DecideReview(reviewer, oldTask.ID, oldTask.Revision, "approve", ""); !errors.Is(err, core.ErrReviewStale) {
		t.Fatalf("withdrawn submission approved: %v", err)
	}
	update = submit(author, article.ID, update.SubmissionID, ms.ColumnArticle, "New title", "new public body")
	decide(taskFor(ms.ReviewColumn, update.SubmissionID), "approve")
	public, err = s.GetColumn(nil, article.ID, false)
	must(err)
	if public.Title != "New title" || public.PublishedOn != firstPublication || !public.Official {
		t.Fatalf("publication identity/marker lost: %+v", public)
	}
	if _, err := s.SubmitColumn(manager, article.ID, update.SubmissionID, ms.ColumnArticle, &ms.ColumnSubmission{Title: "Hijack", Body: "body"}); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("manager edited author body: %v", err)
	}
	// The operator override must not bypass the no-self-review rule either.
	opArticle := submit(operator, 0, 0, ms.ColumnArticle, "Operator", "body")
	if opArticle.Submission.AuditStatus != ms.PostAuditApproved {
		t.Fatal("operator exemption not applied")
	}
	// Rejected updates leave the latest public edition intact.
	rejected := submit(author, article.ID, update.SubmissionID, ms.ColumnArticle, "Rejected title", "rejected body")
	decide(taskFor(ms.ReviewColumn, rejected.SubmissionID), "reject")
	public, err = s.GetColumn(nil, article.ID, false)
	must(err)
	if public.Title != "New title" {
		t.Fatal("rejection changed public article")
	}
	// A stale actor object cannot publish after the underlying grant is removed.
	pending := submit(author, article.ID, rejected.SubmissionID, ms.ColumnArticle, "Revoked", "revoked body")
	must(db.Where("group_id=? AND permission=?", publish.ID, authz.ColumnPublish).Delete(&dbr.IdentityGroupPermission{}).Error)
	must(db.Transaction(cancelRevokedColumnSubmissionsTx))
	if _, err := review.DecideReview(reviewer, taskFor(ms.ReviewColumn, pending.SubmissionID).ID, 1, "approve", ""); !errors.Is(err, core.ErrReviewStale) {
		t.Fatalf("revoked pending publication approved: %v", err)
	}
	if _, err := s.SubmitColumn(author, 0, 0, ms.ColumnArticle, &ms.ColumnSubmission{Title: "Denied", Body: "body"}); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("stale author grant accepted: %v", err)
	}
	must(db.Create(&dbr.IdentityGroupPermission{GroupID: publish.ID, Permission: authz.ColumnPublish}).Error)
	// A non-owner can curate references only in administrator-created collections.
	collection, err := s.SubmitColumn(author, 0, 0, ms.ColumnCollection, &ms.ColumnSubmission{Title: "Author series", Summary: "Climbing Notes 50%_complete"})
	must(err)
	decide(taskFor(ms.ReviewColumn, collection.SubmissionID), "approve")
	curated := submit(manager, 0, 0, ms.ColumnCollection, "Curated series", "")
	decide(taskFor(ms.ReviewColumn, curated.SubmissionID), "approve")
	_, total, err = s.ListColumns(nil, core.ColumnQuery{Kind: ms.ColumnCollection, Limit: 20})
	must(err)
	if total != 0 {
		t.Fatal("empty collections exposed")
	}
	must(db.Create(&dbr.UserIdentityGroup{UserID: other.ID, GroupID: exempt.ID}).Error)
	must(dbr.LoadUserIdentities(db, other))
	otherArticle := submit(other, 0, 0, ms.ColumnArticle, "Other author", "body")
	if err := s.SetColumnEntries(author, collection.ID, []int64{otherArticle.ID}); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("ordinary cross-author collection: %v", err)
	}
	must(s.SetColumnEntries(author, collection.ID, []int64{article.ID}))
	must(s.SetColumnEntries(manager, curated.ID, []int64{otherArticle.ID, article.ID}))
	entries, err := s.ColumnEntries(nil, curated.ID, false)
	must(err)
	if len(entries) != 2 || entries[0].ID != otherArticle.ID {
		t.Fatal("collection order lost")
	}
	// Public profile queries and collection search must never expose submissions.
	unpublished := submit(author, 0, 0, ms.ColumnCollection, "Unpublished series", "")
	must(s.SetColumnEntries(author, unpublished.ID, []int64{article.ID}))
	_, err = s.SubmitColumn(author, collection.ID, collection.SubmissionID, ms.ColumnCollection, &ms.ColumnSubmission{Title: "Pending collection name", Summary: "Pending introduction"})
	must(err)
	for _, test := range []struct {
		name, kind, keyword string
		authorID            int64
		want                int64
	}{
		{"public author articles", ms.ColumnArticle, "", author.ID, 1},
		{"other author articles", ms.ColumnArticle, "", other.ID, 1},
		{"public author collections", ms.ColumnCollection, "", author.ID, 1},
		{"collection name", ms.ColumnCollection, "AUTHOR SERIES", author.ID, 1},
		{"collection introduction", ms.ColumnCollection, "climbing notes", 0, 1},
		{"literal search characters", ms.ColumnCollection, "%_", 0, 1},
		{"pending collection name", ms.ColumnCollection, "Pending collection name", author.ID, 0},
		{"pending collection introduction", ms.ColumnCollection, "Pending introduction", author.ID, 0},
		{"unpublished collection", ms.ColumnCollection, "Unpublished series", author.ID, 0},
		{"collected article body excluded", ms.ColumnCollection, "body", 0, 0},
		{"author filter", ms.ColumnCollection, "climbing notes", other.ID, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows, total, err := s.ListColumns(nil, core.ColumnQuery{Kind: test.kind, Keyword: test.keyword, AuthorID: test.authorID, Limit: 20})
			must(err)
			if total != test.want || int64(len(rows)) != test.want {
				t.Fatalf("public listing: total=%d rows=%d want=%d", total, len(rows), test.want)
			}
		})
	}
	if err := s.SetColumnEntries(author, curated.ID, nil); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("author removed administrator membership: %v", err)
	}
	// A former administrator cannot clear or delete a curated collection, even
	// when its creator still holds ordinary publication permission.
	must(db.Where("group_id=? AND permission=?", manageGroup.ID, authz.ContentManage).Delete(&dbr.IdentityGroupPermission{}).Error)
	if err := s.SetColumnEntries(manager, curated.ID, nil); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("former manager cleared curated collection: %v", err)
	}
	if err := s.ColumnAction(manager, curated.ID, "delete"); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("former manager deleted curated collection: %v", err)
	}
	must(db.Create(&dbr.IdentityGroupPermission{GroupID: manageGroup.ID, Permission: authz.ContentManage}).Error)
	privateMedia := &dbr.Attachment{Model: &dbr.Model{}, UserID: other.ID, Content: "private-course-resource", Purpose: "course", Verified: true, Type: dbr.AttachmentTypeOther}
	must(db.Create(privateMedia).Error)
	if err := s.CheckColumnMedia(author, 0, []*ms.ColumnContent{{Content: privateMedia.Content, Type: ms.ContentTypeAttachment}}); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("private media reissued: %v", err)
	}
	communityMedia := &dbr.Attachment{Model: &dbr.Model{}, UserID: author.ID, Content: "own-community-resource", Purpose: "community", Verified: true, Type: dbr.AttachmentTypeImage}
	must(db.Create(communityMedia).Error)
	must(s.CheckColumnMedia(author, 0, []*ms.ColumnContent{{Content: communityMedia.Content, Type: ms.ContentTypeImage}}))
	if err := s.CheckColumnMedia(other, 0, []*ms.ColumnContent{{Content: communityMedia.Content, Type: ms.ContentTypeImage}}); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("foreign media reissued: %v", err)
	}
	// Discussion is independent of article versions and follows review visibility.
	comment, err := s.CreateColumnComment(author, &ms.ColumnComment{ColumnID: article.ID, Content: "Comment", Contents: []*ms.ColumnContent{{Type: ms.ContentTypeText, Content: "Comment"}}})
	must(err)
	_, count, err := s.ColumnComments(nil, article.ID, "default", 0, 20)
	must(err)
	if count != 0 {
		t.Fatal("pending comment exposed")
	}
	_, count, err = s.ColumnComments(author, article.ID, "default", 0, 20)
	must(err)
	if count != 1 {
		t.Fatal("own pending comment missing")
	}
	if _, err := s.CreateColumnComment(other, &ms.ColumnComment{ColumnID: article.ID, ParentID: comment.ID, Content: "Hidden parent"}); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("reply to hidden parent accepted: %v", err)
	}
	decide(taskFor(ms.ReviewColumnComment, comment.ID), "approve")
	reply, err := s.CreateColumnComment(other, &ms.ColumnComment{ColumnID: article.ID, ParentID: comment.ID, AtUserID: author.ID, Content: "Reply"})
	must(err)
	if reply.AuditStatus != ms.PostAuditApproved {
		t.Fatal("reply exemption lost")
	}
	must(s.ColumnCommentAction(other, comment.ID, "up"))
	must(s.ColumnCommentAction(other, comment.ID, "down"))
	comments, _, err := s.ColumnComments(other, article.ID, "hots", 0, 20)
	must(err)
	if len(comments) != 1 || comments[0].Likes != 0 || comments[0].Reaction != -1 || len(comments[0].Replies) != 1 {
		t.Fatalf("reaction/reply state: %+v", comments)
	}
	must(s.ColumnCommentAction(manager, comment.ID, "feature"))
	must(s.ColumnAction(author, article.ID, "lock"))
	if _, err := s.CreateColumnComment(other, &ms.ColumnComment{ColumnID: article.ID, Content: "Locked"}); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("locked discussion accepted: %v", err)
	}
	must(s.ColumnAction(author, article.ID, "unlock"))
	if err := s.ColumnCommentAction(author, reply.ID, "delete"); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("article owner deleted other's reply: %v", err)
	}
	must(s.ColumnCommentAction(author, comment.ID, "delete"))
	_, count, err = s.ColumnComments(nil, article.ID, "default", 0, 20)
	must(err)
	if count != 0 {
		t.Fatal("deleted thread still visible")
	}
	var child ms.ColumnComment
	if err := db.First(&child, reply.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("reply not removed with parent")
	}
	t.Run("reply notifications follow public thread visibility", func(t *testing.T) {
		row := submit(operator, 0, 0, ms.ColumnArticle, "Notification timing", "body")
		interactions := func(id int64) int64 {
			t.Helper()
			var count int64
			must(db.Model(&ms.Message{}).Where("column_id=? AND comment_id=? AND sender_user_id>0", row.ID, id).Count(&count).Error)
			return count
		}
		for _, rejectParent := range []bool{false, true} {
			parent, err := s.CreateColumnComment(author, &ms.ColumnComment{ColumnID: row.ID, Content: "pending parent"})
			must(err)
			child, err := s.CreateColumnComment(author, &ms.ColumnComment{ColumnID: row.ID, ParentID: parent.ID, AtUserID: other.ID, Mentions: []string{other.Username}, Content: "pending reply"})
			must(err)
			childTask := taskFor(ms.ReviewColumnComment, child.ID)
			decide(childTask, "approve")
			if got := interactions(child.ID); got != 0 {
				t.Errorf("approved reply beneath pending parent notified %d recipients", got)
			}
			// Exempt replies must also wait for their parent to become public.
			exemptChild, err := s.CreateColumnComment(operator, &ms.ColumnComment{ColumnID: row.ID, ParentID: parent.ID, AtUserID: other.ID, Content: "exempt reply"})
			must(err)
			if got := interactions(exemptChild.ID); got != 0 {
				t.Errorf("exempt reply beneath pending parent notified %d recipients", got)
			}
			pendingChild, err := s.CreateColumnComment(author, &ms.ColumnComment{ColumnID: row.ID, ParentID: parent.ID, AtUserID: other.ID, Content: "still pending reply"})
			must(err)
			action := "approve"
			if rejectParent {
				action = "reject"
			}
			decide(taskFor(ms.ReviewColumnComment, parent.ID), action)
			want := int64(2) // Article owner and at/mentioned user; exclude self and deduplicate.
			if rejectParent {
				want = 0
			}
			if got := interactions(child.ID); got != want {
				t.Errorf("reply notifications after parent %s = %d, want %d", action, got, want)
			}
			if got := interactions(exemptChild.ID); got != want {
				t.Errorf("exempt reply notifications after parent %s = %d, want %d", action, got, want)
			}
			if got := interactions(pendingChild.ID); got != 0 {
				t.Error("pending child notified when parent became public")
			}
			if !rejectParent {
				decide(taskFor(ms.ReviewColumnComment, pendingChild.ID), "approve")
				if got := interactions(pendingChild.ID); got != 2 {
					t.Errorf("reply approved after parent notified %d recipients, want 2", got)
				}
			}
			if _, err := review.DecideReview(operator, childTask.ID, childTask.Revision, "approve", ""); !errors.Is(err, core.ErrReviewStale) {
				t.Fatalf("completed reply reviewed twice: %v", err)
			}
			if got := interactions(child.ID); got != want {
				t.Fatal("stale decision duplicated reply notifications")
			}
		}
	})
	t.Run("approval enforces comment limit atomically", func(t *testing.T) {
		row := submit(operator, 0, 0, ms.ColumnArticle, "Comment limit", "body")
		oldLimit := conf.AppSetting.MaxCommentCount
		conf.AppSetting.MaxCommentCount = 1
		defer func() { conf.AppSetting.MaxCommentCount = oldLimit }()
		first, err := s.CreateColumnComment(author, &ms.ColumnComment{ColumnID: row.ID, Content: "first pending"})
		must(err)
		second, err := s.CreateColumnComment(author, &ms.ColumnComment{ColumnID: row.ID, ParentID: first.ID, Content: "second pending reply"})
		must(err)
		// Approving an otherwise hidden reply still consumes the approved-comment quota.
		decide(taskFor(ms.ReviewColumnComment, second.ID), "approve")
		task := taskFor(ms.ReviewColumnComment, first.ID)
		if _, err := review.DecideReview(operator, task.ID, task.Revision, "approve", ""); !errors.Is(err, authz.ErrInvalid) {
			t.Errorf("approval over comment limit accepted: %v", err)
		}
		var current ms.ColumnComment
		must(db.First(&current, first.ID).Error)
		must(db.First(task, task.ID).Error)
		if current.AuditStatus != ms.PostAuditPending || task.State != ms.ReviewPending {
			t.Error("failed approval changed comment or completed review task")
		}
		var notices int64
		must(db.Model(&ms.Message{}).Where("column_id=? AND comment_id=?", row.ID, first.ID).Count(&notices).Error)
		if notices != 0 {
			t.Error("failed approval sent notifications")
		}
		// Rejection remains available when the limit is full.
		if current.AuditStatus == ms.PostAuditPending {
			decide(task, "reject")
		}
	})
	t.Run("concurrent approvals share the comment quota", func(t *testing.T) {
		row := submit(operator, 0, 0, ms.ColumnArticle, "Concurrent quota", "body")
		oldLimit := conf.AppSetting.MaxCommentCount
		conf.AppSetting.MaxCommentCount = 1
		defer func() { conf.AppSetting.MaxCommentCount = oldLimit }()
		var tasks []*ms.ReviewTask
		for range 2 {
			comment, err := s.CreateColumnComment(author, &ms.ColumnComment{ColumnID: row.ID, Content: "pending"})
			must(err)
			tasks = append(tasks, taskFor(ms.ReviewColumnComment, comment.ID))
		}
		results := make(chan error, 2)
		start := make(chan struct{})
		for _, task := range tasks {
			go func() {
				<-start
				_, err := review.DecideReview(operator, task.ID, task.Revision, "approve", "")
				results <- err
			}()
		}
		close(start)
		approved, refused := 0, 0
		for range 2 {
			err := <-results
			if err == nil {
				approved++
			} else if errors.Is(err, authz.ErrInvalid) {
				refused++
			} else {
				t.Fatalf("unexpected concurrent decision failure: %v", err)
			}
		}
		if approved != 1 || refused != 1 {
			t.Fatalf("concurrent approvals=%d refusals=%d", approved, refused)
		}
	})
	// Takedown dominates direct reads, search, collection entries and publication.
	must(s.ColumnAction(manager, article.ID, "takedown"))
	if _, err := s.GetColumn(nil, article.ID, false); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("taken-down detail exposed: %v", err)
	}
	entries, err = s.ColumnEntries(nil, curated.ID, false)
	must(err)
	if len(entries) != 1 {
		t.Fatal("taken-down collection entry exposed")
	}
	if err := s.ColumnAction(author, article.ID, "restore"); !errors.Is(err, authz.ErrDenied) {
		t.Fatal("author restored management takedown")
	}
	must(s.ColumnAction(manager, article.ID, "restore"))
	must(s.ColumnAction(author, collection.ID, "delete"))
	if _, err := s.GetColumn(nil, article.ID, false); err != nil {
		t.Fatal("deleting collection deleted article")
	}
	longReasonArticle := submit(author, 0, 0, ms.ColumnArticle, "Long reason", "body")
	longReasonTask := taskFor(ms.ReviewColumn, longReasonArticle.SubmissionID)
	_, err = review.DecideReview(operator, longReasonTask.ID, longReasonTask.Revision, "reject", strings.Repeat("理", 255))
	must(err)
	must(review.ReconcileReviewTasks(48 * time.Hour))
	// Administrator deletion returns only the latest submission to its author.
	beforeDelete, err := s.GetColumn(author, article.ID, true)
	must(err)
	file := &dbr.Attachment{Model: &dbr.Model{}, UserID: author.ID, Content: "returned-community-file", Purpose: "community", Verified: true, Type: dbr.AttachmentTypeOther}
	must(db.Create(file).Error)
	latest, err := s.SubmitColumn(author, article.ID, beforeDelete.SubmissionID, ms.ColumnArticle, &ms.ColumnSubmission{
		Title: "Latest returned submission", Body: "latest private returned content", Attachments: []*ms.ColumnContent{{Content: file.Content, Type: ms.ContentTypeAttachment}},
	})
	must(err)
	deletedTask := taskFor(ms.ReviewColumn, latest.SubmissionID)
	if err := s.DeleteColumn(manager, article.ID, " "); !errors.Is(err, authz.ErrInvalid) {
		t.Fatalf("administrator deletion accepted an empty reason: %v", err)
	}
	if err := s.DeleteColumn(manager, article.ID, strings.Repeat("中", 256)); !errors.Is(err, authz.ErrInvalid) {
		t.Fatalf("administrator deletion accepted an oversized reason: %v", err)
	}
	must(s.DeleteColumn(manager, article.ID, "Please revise the content"))
	returned, err := s.GetReturnedColumn(author, article.ID)
	must(err)
	if returned.Body != "latest private returned content" || returned.Title != "Latest returned submission" || returned.SubmissionID != latest.SubmissionID || len(returned.Attachments) != 1 || returned.AdminDeletedBy != manager.ID || returned.AdminDeleteReason != "Please revise the content" {
		t.Fatalf("returned content is not the latest submitted version: %+v", returned)
	}
	for _, actor := range []*ms.User{nil, other, manager, operator} {
		if _, err := s.GetReturnedColumn(actor, article.ID); err == nil {
			t.Fatal("returned content exposed to someone other than its author")
		}
	}
	if _, err := s.GetColumn(author, article.ID, true); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("normal submission endpoint returned deleted content: %v", err)
	}
	if _, err := s.ColumnAttachment(author, article.ID, latest.SubmissionID, 0); err == nil {
		t.Fatal("normal attachment endpoint accepted a deleted article")
	}
	if _, err := review.DecideReview(reviewer, deletedTask.ID, deletedTask.Revision, "approve", ""); !errors.Is(err, core.ErrReviewStale) {
		t.Fatalf("deleted submission remained actionable: %v", err)
	}
	rows, total, err := s.ListColumns(author, core.ColumnQuery{Kind: ms.ColumnArticle, Mine: true, Returned: true, Keyword: "Latest returned", Limit: 20})
	must(err)
	if total != 1 || len(rows) != 1 || rows[0].Title != returned.Title {
		t.Fatal("author return list did not use latest submitted metadata")
	}
	_, total, err = s.ListColumns(other, core.ColumnQuery{Kind: ms.ColumnArticle, Mine: true, Returned: true, Limit: 20})
	must(err)
	if total != 0 {
		t.Fatal("another author could list returned content")
	}
	if _, _, err := s.ListColumns(nil, core.ColumnQuery{Kind: ms.ColumnArticle, Mine: true, Returned: true, Limit: 20}); err == nil {
		t.Fatal("guest could list returned content")
	}
	var notices int64
	countNotices := func() {
		must(db.Model(&ms.Message{}).Where("column_id=? AND receiver_user_id=? AND column_returned", article.ID, author.ID).Count(&notices).Error)
		if notices != 1 {
			t.Fatalf("return notifications=%d, want one", notices)
		}
	}
	countNotices()
	if err := s.DeleteColumn(manager, article.ID, "Repeated"); err == nil {
		t.Fatal("deleted article could be deleted twice")
	}
	countNotices()
	must(db.Where("group_id=? AND permission=?", publish.ID, authz.ColumnPublish).Delete(&dbr.IdentityGroupPermission{}).Error)
	_, err = s.GetReturnedColumn(author, article.ID)
	must(err)
	if _, err := s.RecreateColumn(author, article.ID, &ms.ColumnSubmission{Title: "Denied", Body: "Text"}); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("revoked publisher could recreate returned content: %v", err)
	}
	must(db.Create(&dbr.IdentityGroupPermission{GroupID: publish.ID, Permission: authz.ColumnPublish}).Error)
	must(db.Create(&dbr.UserIdentityGroup{UserID: author.ID, GroupID: exempt.ID}).Error)
	if _, err := s.RecreateColumn(operator, article.ID, &ms.ColumnSubmission{Title: "Not mine", Body: "Text"}); err == nil {
		t.Fatal("operator recreated someone else's returned content")
	}
	recreated, err := s.RecreateColumn(author, article.ID, &ms.ColumnSubmission{Title: "Recreated article", Body: returned.Body, Attachments: returned.Attachments})
	must(err)
	if recreated.ID == article.ID || recreated.PublishedOn != 0 || recreated.Submission.AuditStatus != ms.PostAuditPending || recreated.Official || recreated.Locked || recreated.RecreatedFromID != article.ID {
		t.Fatalf("recreated content bypassed review or inherited original state: %+v", recreated)
	}
	must(s.ColumnAction(author, recreated.ID, "withdraw"))
	resubmitted := submit(author, recreated.ID, recreated.SubmissionID, ms.ColumnArticle, "Resubmitted recreation", "Revised returned content")
	if resubmitted.Submission.AuditStatus != ms.PostAuditPending {
		t.Fatal("withdraw/resubmit bypassed mandatory first-publication review")
	}
	decide(taskFor(ms.ReviewColumn, resubmitted.SubmissionID), "approve")
	_, err = s.GetColumn(nil, recreated.ID, false)
	must(err)
	if _, err := s.GetColumn(nil, article.ID, false); err == nil {
		t.Fatal("recreating restored the original article")
	}
	selfDeleted := submit(author, 0, 0, ms.ColumnArticle, "Self deleted", "Text")
	must(s.ColumnAction(author, selfDeleted.ID, "delete"))
	if _, err := s.GetReturnedColumn(author, selfDeleted.ID); err == nil {
		t.Fatal("author deletion unexpectedly created a return record")
	}
}
