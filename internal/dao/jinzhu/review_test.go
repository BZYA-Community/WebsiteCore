package jinzhu

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"
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

type reviewQueryTrace struct {
	logger.Interface
	trace func(string, int64)
}

func (l reviewQueryTrace) Trace(ctx context.Context, begin time.Time, result func() (string, int64), err error) {
	query, rows := result()
	l.trace(query, rows)
	l.Interface.Trace(ctx, begin, result, err)
}

func TestReviewAssignmentsAndAtomicDecisions(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; PostgreSQL review integration test")
	}
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		t.Fatal(err)
	}
	prefix := "ut_review_" + hex.EncodeToString(bytes) + "_"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix, SingularTable: true}})
	if err != nil {
		t.Fatal(err)
	}
	models := []any{&dbr.User{}, &dbr.IdentityGroup{}, &dbr.IdentityGroupPermission{}, &dbr.UserIdentityGroup{},
		&dbr.Post{}, &dbr.PostContent{}, &dbr.PostMetric{}, &dbr.Tag{}, &dbr.PostStar{}, &dbr.PostCollection{}, &dbr.TweetCommentThumbs{}, &dbr.Comment{}, &dbr.CommentContent{}, &dbr.CommentReply{},
		&dbr.Course{}, &dbr.CourseComment{}, &dbr.CourseCommentContent{}, &dbr.CourseCommentReply{}, &dbr.ReviewTask{}, &dbr.ReviewTaskEvent{}, &dbr.AuditLog{}, &dbr.Attachment{}, &dbr.OperationLog{}}
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
	if err := db.Exec("CREATE UNIQUE INDEX " + prefix + "tag_name ON " + prefix + "tag (tag)").Error; err != nil {
		t.Fatal(err)
	}
	oldAudit := conf.AuditSetting
	conf.AuditSetting = &conf.AuditConf{Enabled: true, DeadlineHours: 48}
	t.Cleanup(func() { conf.AuditSetting = oldAudit })
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	group := func(key string, permissions ...string) *dbr.IdentityGroup {
		t.Helper()
		g := &dbr.IdentityGroup{Key: key, Name: key}
		must(db.Create(g).Error)
		for _, permission := range permissions {
			must(db.Create(&dbr.IdentityGroupPermission{GroupID: g.ID, Permission: permission}).Error)
		}
		return g
	}
	guestGroup := group("guest", authz.PostView, authz.PostCreate, authz.CommentCreate, authz.CourseView, authz.ProfileEdit, authz.CommunityInteract, authz.ContentUpload)
	reviewerGroup := group("reviewer", authz.ContentReview)
	viewerGroup := group("all_tasks", authz.AuditViewAll)
	user := func(name string, groups ...*dbr.IdentityGroup) *ms.User {
		t.Helper()
		u := &ms.User{Model: &ms.Model{}, Username: name, Nickname: name, Status: ms.UserStatusNormal}
		must(db.Create(u).Error)
		for _, group := range groups {
			must(db.Create(&dbr.UserIdentityGroup{UserID: u.ID, GroupID: group.ID}).Error)
		}
		must(dbr.LoadUserIdentities(db, u))
		return u
	}
	author := user("author")
	reviewerA := user("reviewer_a", reviewerGroup)
	ordinaryUsers := make([]ms.User, 199)
	for i := range ordinaryUsers {
		ordinaryUsers[i] = ms.User{Model: &ms.Model{}, Username: fmt.Sprintf("ordinary_%d", i), Status: ms.UserStatusNormal}
	}
	must(db.CreateInBatches(&ordinaryUsers, 100).Error)
	s := &reviewSrv{db: db}
	submitProfile := func(user *ms.User, kind, value string) error {
		_, err := s.SubmitProfileReview(user, kind, value)
		return err
	}
	taskFor := func(kind string, id int64) *ms.ReviewTask {
		t.Helper()
		var task ms.ReviewTask
		must(db.Where("kind = ? AND target_id = ?", kind, id).First(&task).Error)
		return &task
	}
	// Measure reviewer lookup after the submission event. Before the fix this
	// loaded all 201 accounts and resolved their identities with three more reads.
	var submitted bool
	var lookupQueries int
	var lookupRows int64
	submitDB := db.Session(&gorm.Session{Logger: reviewQueryTrace{Interface: db.Logger, trace: func(query string, rows int64) {
		if submitted && strings.HasPrefix(query, "SELECT ") {
			lookupQueries++
			lookupRows += rows
		}
		if strings.HasPrefix(query, "INSERT INTO ") && strings.Contains(query, `"`+prefix+`review_task_event"`) && strings.Contains(query, "'submitted'") {
			submitted = true
		}
	}}})
	post, err := (&reviewSrv{db: submitDB}).CreatePostWithContents(&ms.Post{UserID: author.ID, Visibility: ms.PostVisitPublic, Tags: "review"}, []*ms.PostContent{{Type: ms.ContentTypeText, Content: "Complete content"}})
	must(err)
	if lookupQueries != 1 || lookupRows != 1 {
		t.Fatalf("reviewer lookup returned %d rows in %d queries, want 1 row in 1 query", lookupRows, lookupQueries)
	}
	task := taskFor(ms.ReviewPost, post.ID)
	if task.AssigneeID != reviewerA.ID || task.DeadlineOn-task.AssignedOn != int64(48*time.Hour/time.Second) {
		t.Fatalf("assignment/deadline: %+v", task)
	}

	reviewerB := user("reviewer_b", reviewerGroup)
	viewer := user("viewer", viewerGroup)
	actorFor := func(task *ms.ReviewTask) *ms.User {
		t.Helper()
		if task.AssigneeID == reviewerA.ID {
			return reviewerA
		}
		if task.AssigneeID == reviewerB.ID {
			return reviewerB
		}
		t.Fatalf("unexpected assignee: %d", task.AssigneeID)
		return nil
	}
	assigned, other := reviewerA, reviewerB
	if _, err := s.DecideReview(other, task.ID, task.Revision, "approve", ""); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("unassigned decision: %v", err)
	}
	if _, err := s.DecideReview(viewer, task.ID, task.Revision, "approve", ""); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("view-all decision: %v", err)
	}
	listed, total, err := s.ListReviewTasks(other, ms.ReviewPost, ms.ReviewPending, 0, 20)
	must(err)
	if total != 0 || len(listed) != 0 {
		t.Fatal("unassigned reviewer saw task")
	}
	_, total, err = s.ListReviewTasks(viewer, ms.ReviewPost, ms.ReviewPending, 0, 20)
	must(err)
	if total != 1 {
		t.Fatal("view-all cannot see pending task")
	}
	if _, err := s.DecideReview(assigned, task.ID, task.Revision, "approve", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DecideReview(assigned, task.ID, task.Revision, "approve", ""); !errors.Is(err, core.ErrReviewStale) {
		t.Fatalf("duplicate review: %v", err)
	}
	var tag dbr.Tag
	must(db.Where("tag = ?", "review").First(&tag).Error)
	if tag.QuoteNum != 1 {
		t.Fatal("tag count not committed")
	}
	// Submission failure cannot leave metadata, body, or an unassigned orphan.
	var beforePosts, afterPosts int64
	must(db.Model(&ms.Post{}).Count(&beforePosts).Error)
	eventConstraint := prefix + "deny_event"
	must(db.Exec("ALTER TABLE " + prefix + "review_task_event ADD CONSTRAINT " + eventConstraint + " CHECK (false) NOT VALID").Error)
	if _, err := s.CreatePostWithContents(&ms.Post{UserID: author.ID, Visibility: ms.PostVisitPublic}, []*ms.PostContent{{Type: ms.ContentTypeText, Content: "Rollback body"}}); err == nil {
		t.Fatal("ignored failed submission event")
	}
	must(db.Model(&ms.Post{}).Count(&afterPosts).Error)
	if beforePosts != afterPosts {
		t.Fatal("failed submission left post metadata")
	}
	must(db.Exec("ALTER TABLE " + prefix + "review_task_event DROP CONSTRAINT " + eventConstraint).Error)

	comment, err := s.CreateCommentWithContents(&ms.Comment{PostID: post.ID, UserID: author.ID}, []*ms.CommentContent{{Type: ms.ContentTypeText, Content: "Question"}})
	must(err)
	commentTask := taskFor(ms.ReviewComment, comment.ID)
	// A failing audit write must roll back the status, parent count and task event.
	constraint := prefix + "deny_audit"
	must(db.Exec("ALTER TABLE " + prefix + "audit_log ADD CONSTRAINT " + constraint + " CHECK (false) NOT VALID").Error)
	if _, err := s.DecideReview(actorFor(commentTask), commentTask.ID, commentTask.Revision, "approve", ""); err == nil {
		t.Fatal("ignored failed audit log")
	}
	must(db.First(comment, comment.ID).Error)
	must(db.First(post, post.ID).Error)
	if comment.AuditStatus != ms.PostAuditPending || post.CommentCount != 0 || taskFor(ms.ReviewComment, comment.ID).State != ms.ReviewPending {
		t.Fatal("review did not roll back atomically")
	}
	must(db.Exec("ALTER TABLE " + prefix + "audit_log DROP CONSTRAINT " + constraint).Error)
	_, err = s.DecideReview(actorFor(commentTask), commentTask.ID, commentTask.Revision, "approve", "")
	must(err)
	reply, err := (&commentManageSrv{db: db}).CreateCommentReply(&ms.CommentReply{CommentID: comment.ID, UserID: author.ID, Content: "Reply"})
	must(err)
	replyTask := taskFor(ms.ReviewReply, reply.ID)
	_, err = s.DecideReview(actorFor(replyTask), replyTask.ID, replyTask.Revision, "approve", "")
	must(err)
	must(db.First(comment, comment.ID).Error)
	must(db.First(post, post.ID).Error)
	if comment.ReplyCount != 1 || post.CommentCount != 2 {
		t.Fatalf("comment counts %d/%d", comment.ReplyCount, post.CommentCount)
	}

	course := &ms.Course{Model: &ms.Model{}, Title: "Course", TeacherID: author.ID}
	must(db.Create(course).Error)
	question, err := s.CreateCourseQuestionWithContents(&ms.CourseComment{CourseID: course.ID, UserID: author.ID}, []*ms.CourseCommentContent{{Type: ms.ContentTypeText, Content: "Course question"}})
	must(err)
	questionTask := taskFor(ms.ReviewCourseQuestion, question.ID)
	_, err = s.DecideReview(actorFor(questionTask), questionTask.ID, questionTask.Revision, "approve", "")
	must(err)
	answer, err := (&courseManageSrv{db: db}).CreateCourseCommentReply(&ms.CourseCommentReply{CommentID: question.ID, UserID: author.ID, Content: "Answer"})
	must(err)
	answerTask := taskFor(ms.ReviewCourseAnswer, answer.ID)
	_, err = s.DecideReview(actorFor(answerTask), answerTask.ID, answerTask.Revision, "approve", "")
	must(err)
	must(db.First(question, question.ID).Error)
	must(db.First(course, course.ID).Error)
	if question.ReplyCount != 1 || course.CommentCount != 2 {
		t.Fatal("course review counters")
	}
	// The request may have read an accessible parent before another request
	// made it private or rejected it. The write transaction must recheck it.
	must(db.Model(post).Update("visibility", ms.PostVisitPrivate).Error)
	if _, err := s.CreateCommentWithContents(&ms.Comment{PostID: post.ID, UserID: reviewerA.ID}, nil); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("comment on newly private parent: %v", err)
	}
	if _, err := (&tweetManageSrv{db: db}).CreatePostStar(post.ID, reviewerA.ID); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("reaction on newly private parent: %v", err)
	}
	must(db.Model(post).Update("visibility", ms.PostVisitPublic).Error)
	must(db.Model(comment).Update("audit_status", ms.PostAuditRejected).Error)
	if _, err := (&commentManageSrv{db: db}).CreateCommentReply(&ms.CommentReply{CommentID: comment.ID, UserID: reviewerA.ID, Content: "Hidden parent"}); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("reply on newly hidden comment: %v", err)
	}
	must(db.Model(comment).Update("audit_status", ms.PostAuditApproved).Error)
	must(db.Model(question).Update("audit_status", ms.PostAuditRejected).Error)
	if _, err := (&courseManageSrv{db: db}).CreateCourseCommentReply(&ms.CourseCommentReply{CommentID: question.ID, UserID: reviewerA.ID, Content: "Hidden question"}); !errors.Is(err, authz.ErrDenied) {
		t.Fatalf("answer on newly hidden question: %v", err)
	}
	must(db.Model(question).Update("audit_status", ms.PostAuditApproved).Error)
	audit := &auditSrv{db: db}
	rows, total, err := audit.ListAuditComments(viewer, -1, 0, 20)
	must(err)
	if total != 4 || len(rows) != 4 {
		t.Fatalf("combined task queue: %d/%d", total, len(rows))
	}
	rows, _, err = audit.ListAuditComments(reviewerA, -1, 0, 20)
	must(err)
	for _, row := range rows {
		kinds := []string{ms.ReviewComment, ms.ReviewReply, ms.ReviewCourseQuestion, ms.ReviewCourseAnswer}
		if taskFor(kinds[row.CommentType], row.ID).AssigneeID != reviewerA.ID {
			t.Fatal("combined queue escaped assignment scope")
		}
	}

	rejected, err := s.CreatePostWithContents(&ms.Post{UserID: author.ID, Visibility: ms.PostVisitPublic}, []*ms.PostContent{{Type: ms.ContentTypeText, Content: "Resubmit"}})
	must(err)
	rejectedTask := taskFor(ms.ReviewPost, rejected.ID)
	_, err = s.DecideReview(actorFor(rejectedTask), rejectedTask.ID, rejectedTask.Revision, "reject", "Please revise")
	must(err)
	must(db.First(rejected, rejected.ID).Error)
	if rejected.Visibility != ms.PostVisitPrivate || rejected.AuditStatus != ms.PostAuditRejected {
		t.Fatal("rejection did not hide post")
	}
	_, err = s.SetPostVisibilityForReview(author, rejected.ID, ms.PostVisitPublic)
	must(err)
	resubmittedTask := taskFor(ms.ReviewPost, rejected.ID)
	if resubmittedTask.Revision != rejectedTask.Revision+1 || resubmittedTask.State != ms.ReviewPending {
		t.Fatal("resubmit did not create new revision")
	}
	if _, err := s.DecideReview(actorFor(resubmittedTask), rejectedTask.ID, rejectedTask.Revision, "approve", ""); !errors.Is(err, core.ErrReviewStale) {
		t.Fatal("old submission can approve a new revision")
	}
	// Two simultaneous decisions must never double-count a publication.
	decisions := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := s.DecideReview(actorFor(resubmittedTask), resubmittedTask.ID, resubmittedTask.Revision, "approve", "")
			decisions <- err
		}()
	}
	one, two := <-decisions, <-decisions
	if !((one == nil && errors.Is(two, core.ErrReviewStale)) || (two == nil && errors.Is(one, core.ErrReviewStale))) {
		t.Fatalf("concurrent decisions: %v/%v", one, two)
	}
	stalePost := &ms.Post{Model: &ms.Model{ID: post.ID}, UserID: author.ID, AuditStatus: ms.PostAuditPending, Visibility: ms.PostVisitPrivate, IsLock: 1}
	must(stalePost.Update(db, "is_lock"))
	if err := stalePost.Update(db, "audit_status"); !errors.Is(err, gorm.ErrInvalidData) {
		t.Fatal("ordinary post update accepts audit state")
	}
	must(db.First(post, post.ID).Error)
	if post.AuditStatus != ms.PostAuditApproved || post.Visibility != ms.PostVisitPublic || post.CommentCount != 2 {
		t.Fatal("stale flag update reverted moderation/counters")
	}
	post.IsLock = 0
	must(post.Update(db, "is_lock"))
	reactionErrors := make(chan error, 8)
	for range 8 {
		go func() { _, err := (&tweetManageSrv{db: db}).CreatePostStar(post.ID, author.ID); reactionErrors <- err }()
	}
	for range 8 {
		must(<-reactionErrors)
	}
	must(db.First(post, post.ID).Error)
	if post.UpvoteCount != 1 || post.AuditStatus != ms.PostAuditApproved || post.CommentCount != 2 {
		t.Fatal("concurrent reaction lost or overwrote unrelated post state")
	}
	collectionA, err := (&tweetManageSrv{db: db}).CreatePostCollection(post.ID, author.ID)
	must(err)
	_, err = (&tweetManageSrv{db: db}).CreatePostCollection(post.ID, reviewerA.ID)
	must(err)
	must((&tweetManageSrv{db: db}).DeletePostCollection(collectionA))
	must(db.First(post, post.ID).Error)
	if post.CollectionCount != 1 {
		t.Fatal("collection counter did not reflect committed rows")
	}
	deleteComment, err := s.CreateCommentWithContents(&ms.Comment{PostID: post.ID, UserID: author.ID}, []*ms.CommentContent{{Type: ms.ContentTypeText, Content: "Delete after review"}})
	must(err)
	deleteTask := taskFor(ms.ReviewComment, deleteComment.ID)
	_, err = s.DecideReview(actorFor(deleteTask), deleteTask.ID, deleteTask.Revision, "approve", "")
	must(err)
	pendingReply, err := (&commentManageSrv{db: db}).CreateCommentReply(&ms.CommentReply{CommentID: deleteComment.ID, UserID: author.ID, Content: "Pending child"})
	must(err)
	// deleteComment still carries pending status from before the concurrent approval.
	must((&commentManageSrv{db: db}).DeleteComment(deleteComment))
	must(db.First(post, post.ID).Error)
	if post.CommentCount != 2 || taskFor(ms.ReviewReply, pendingReply.ID).State != ms.ReviewCancelled {
		t.Fatal("stale deletion lost count/cancelled task consistency")
	}

	must(submitProfile(author, ms.ReviewNickname, "First name"))
	oldTask := taskFor(ms.ReviewNickname, author.ID)
	must(submitProfile(author, ms.ReviewNickname, "Second name"))
	newTask := taskFor(ms.ReviewNickname, author.ID)
	if newTask.Revision <= oldTask.Revision {
		t.Fatal("replacement did not revise task")
	}
	if _, err := s.DecideReview(actorFor(newTask), oldTask.ID, oldTask.Revision, "approve", ""); !errors.Is(err, core.ErrReviewStale) {
		t.Fatalf("old snapshot approval: %v", err)
	}
	_, err = s.DecideReview(actorFor(newTask), newTask.ID, newTask.Revision, "approve", "")
	must(err)
	must(db.First(author, author.ID).Error)
	if author.Nickname != "Second name" || author.PendingNickname != "" {
		t.Fatal("wrong nickname approved")
	}
	// A stale exemption in the request must not publish a profile change.
	author.Permissions = append(author.Permissions, authz.PublishUnreviewed)
	profileResult, err := s.SubmitProfileReview(author, ms.ReviewNickname, "Needs review")
	must(err)
	if !profileResult.Pending {
		t.Fatal("stale exemption bypassed profile review")
	}
	publisherGroup := group("publisher", authz.PublishUnreviewed)
	must(db.Create(&dbr.UserIdentityGroup{UserID: author.ID, GroupID: publisherGroup.ID}).Error)
	profileResult, err = s.SubmitProfileReview(author, ms.ReviewNickname, "Fresh exemption")
	must(err)
	if profileResult.Pending || taskFor(ms.ReviewNickname, author.ID).State != ms.ReviewCancelled {
		t.Fatal("current exemption failed to publish/cancel old task")
	}
	must(db.Where("user_id = ? AND group_id = ?", author.ID, publisherGroup.ID).Delete(&dbr.UserIdentityGroup{}).Error)
	operationConstraint := prefix + "deny_operation"
	must(db.Exec("ALTER TABLE " + prefix + "operation_log ADD CONSTRAINT " + operationConstraint + " CHECK (false) NOT VALID").Error)
	if _, err := (&tweetManageSrv{db: db}).CreateAttachment(&ms.Attachment{UserID: author.ID, Name: "fixture.txt", FileSize: 4, Purpose: "community", Verified: true}); err == nil {
		t.Fatal("attachment ignored failed operation log")
	}
	var attachmentCount int64
	must(db.Model(&ms.Attachment{}).Count(&attachmentCount).Error)
	if attachmentCount != 0 {
		t.Fatal("attachment persisted without required log")
	}
	must(db.Exec("ALTER TABLE " + prefix + "operation_log DROP CONSTRAINT " + operationConstraint).Error)

	must(submitProfile(author, ms.ReviewAvatar, "https://fixture/avatar.png"))
	avatarTask := taskFor(ms.ReviewAvatar, author.ID)
	firstReviewer := actorFor(avatarTask)
	must(db.Model(avatarTask).Update("deadline_on", time.Now().Unix()-1).Error)
	if _, err := s.DecideReview(firstReviewer, avatarTask.ID, avatarTask.Revision, "approve", ""); !errors.Is(err, authz.ErrDenied) {
		t.Fatal("late reviewer can decide")
	}
	must(s.ReconcileReviewTasks(48 * time.Hour))
	avatarTask = taskFor(ms.ReviewAvatar, author.ID)
	if avatarTask.AssigneeID == firstReviewer.ID || avatarTask.AssigneeID == 0 {
		t.Fatal("timeout did not transfer to different reviewer")
	}
	var misses int64
	must(db.Model(&ms.ReviewTaskEvent{}).Where("task_id = ? AND event = ?", avatarTask.ID, "timeout").Count(&misses).Error)
	if misses != 1 {
		t.Fatal("missing timeout event")
	}
	must(s.ReconcileReviewTasks(48 * time.Hour))
	must(db.Model(&ms.ReviewTaskEvent{}).Where("task_id = ? AND event = ?", avatarTask.ID, "timeout").Count(&misses).Error)
	if misses != 1 {
		t.Fatal("duplicate timeout counted")
	}
	// A revoked permission invalidates an already-issued assignment immediately.
	secondReviewer := actorFor(avatarTask)
	must(db.Where("user_id = ? AND group_id = ?", secondReviewer.ID, reviewerGroup.ID).Delete(&dbr.UserIdentityGroup{}).Error)
	if _, err := s.DecideReview(secondReviewer, avatarTask.ID, avatarTask.Revision, "approve", ""); !errors.Is(err, authz.ErrDenied) {
		t.Fatal("revoked reviewer retained authority")
	}
	must(s.ReconcileReviewTasks(48 * time.Hour))
	avatarTask = taskFor(ms.ReviewAvatar, author.ID)
	if avatarTask.AssigneeID != firstReviewer.ID {
		t.Fatal("ineligible reviewer was not replaced")
	}
	// Only the current reviewer remains; expiration leaves the task unassigned.
	must(db.Model(avatarTask).Update("deadline_on", time.Now().Unix()-1).Error)
	must(s.ReconcileReviewTasks(48 * time.Hour))
	avatarTask = taskFor(ms.ReviewAvatar, author.ID)
	if avatarTask.AssigneeID != 0 {
		t.Fatal("expired task returned to same reviewer")
	}
	must(s.ReconcileReviewTasks(48 * time.Hour))
	must(db.Model(&ms.ReviewTaskEvent{}).Where("task_id = ? AND event = ?", avatarTask.ID, "timeout").Count(&misses).Error)
	if misses != 2 {
		t.Fatal("unassigned task counted a false extra miss")
	}
	operator := user("operator")
	must(db.Model(operator).Update("is_operator", true).Error)
	_, err = s.DecideReview(operator, avatarTask.ID, avatarTask.Revision, "reject", "Not suitable")
	must(err)
	var overrides int64
	must(db.Model(&ms.ReviewTaskEvent{}).Where("task_id = ? AND event = ?", avatarTask.ID, "operator_override").Count(&overrides).Error)
	if overrides != 1 {
		t.Fatal("operator override was not logged")
	}

	// The two remaining reviewers are excluded when they authored or last held the task.
	excluded := &ms.ReviewTask{AuthorID: firstReviewer.ID, PreviousAssigneeID: operator.ID}
	must(assignReviewTask(db, excluded, 48*time.Hour, time.Now().Unix()))
	if excluded.AssigneeID != 0 {
		t.Fatalf("assigned to author or previous assignee: %d", excluded.AssigneeID)
	}

	additionalGroup := group("additional_reviewer", authz.ContentReview)
	must(db.Create(&dbr.UserIdentityGroup{UserID: firstReviewer.ID, GroupID: additionalGroup.ID}).Error)
	memberGroup := group("member")
	closed := user("closed_reviewer", reviewerGroup)
	must(db.Model(closed).Update("status", ms.UserStatusClosed).Error)
	deleted := user("deleted_operator")
	must(db.Model(deleted).Updates(map[string]any{"is_operator": true, "is_del": 1}).Error)
	emailUser := user("email_user", guestGroup)
	must(db.Model(emailUser).Update("email", "email@example.test").Error)
	phoneUser := user("phone_user", memberGroup, guestGroup)
	must(db.Model(phoneUser).Update("phone", "5550000").Error)

	oldVerification := conf.AccountVerifySetting
	t.Cleanup(func() { conf.AccountVerifySetting = oldVerification })
	check := func(mode string, want ...int64) {
		t.Helper()
		conf.AccountVerifySetting = &conf.AccountVerifyConf{Mode: mode}
		var ids []int64
		must(dbr.EligibleReviewers(db).Order("id").Pluck("id", &ids).Error)
		if !slices.Equal(ids, want) {
			t.Fatalf("%s reviewer eligibility: got %v, want %v", mode, ids, want)
		}
	}
	check("email", firstReviewer.ID, operator.ID)
	must(db.Create(&dbr.IdentityGroupPermission{GroupID: memberGroup.ID, Permission: authz.ContentReview}).Error)
	check("email", firstReviewer.ID, operator.ID, emailUser.ID)
	check("phone", firstReviewer.ID, operator.ID, phoneUser.ID)

	// A stale guest membership must not grant guest permissions to a member.
	must(db.Create(&dbr.IdentityGroupPermission{GroupID: guestGroup.ID, Permission: authz.ContentReview}).Error)
	must(db.Where("group_id = ? AND permission = ?", memberGroup.ID, authz.ContentReview).Delete(&dbr.IdentityGroupPermission{}).Error)
	var guestIDs []int64
	must(dbr.EligibleReviewers(db).Where("id IN ?", []int64{emailUser.ID, phoneUser.ID}).Pluck("id", &guestIDs).Error)
	if !slices.Equal(guestIDs, []int64{emailUser.ID}) {
		t.Fatalf("phone-mode guest reviewer eligibility: got %v, want [%d]", guestIDs, emailUser.ID)
	}
}
