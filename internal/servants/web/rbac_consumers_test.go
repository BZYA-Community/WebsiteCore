package web

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	model "github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
)

func permissionUser(id int64, permissions ...string) *ms.User {
	return &ms.User{Model: &ms.Model{ID: id}, Status: ms.UserStatusNormal, Permissions: permissions}
}

type permissionData struct {
	core.DataService
	user       *ms.User
	parent     *ms.CourseComment
	incoming   bool
	outgoing   bool
	queryError error
	question   *ms.CourseComment
	answer     *ms.CourseCommentReply
	course     *ms.Course
}

func (d *permissionData) HasWhispered(sender, receiver int64) (bool, error) {
	if sender == 2 {
		return d.incoming, d.queryError
	}
	return d.outgoing, d.queryError
}

func (d *permissionData) GetUserByID(int64) (*ms.User, error) { return d.user, nil }
func (d *permissionData) GetCourseByID(int64) (*ms.Course, error) {
	return &ms.Course{Model: &ms.Model{ID: 3}, TeacherID: 1}, nil
}
func (d *permissionData) GetCourseGroupByID(int64) (*ms.CourseGroup, error) {
	return &ms.CourseGroup{Model: &ms.Model{ID: 6}}, nil
}
func (d *permissionData) CreateCourse(_ *ms.User, c *ms.Course) (*ms.Course, error) {
	c.Model = &ms.Model{ID: 3}
	d.course = c
	return c, nil
}
func (d *permissionData) GetCourseCommentByID(int64) (*ms.CourseComment, error) {
	return d.parent, nil
}
func (d *permissionData) CreateCourseQuestionWithContents(c *ms.CourseComment, _ []*ms.CourseCommentContent) (*ms.CourseComment, error) {
	c.Model = &ms.Model{ID: 4}
	d.question = c
	return c, nil
}
func (d *permissionData) CreateCourseCommentContent(c *ms.CourseCommentContent) (*ms.CourseCommentContent, error) {
	return c, nil
}
func (d *permissionData) CreateCourseCommentReply(r *ms.CourseCommentReply) (*ms.CourseCommentReply, error) {
	r.Model = &ms.Model{ID: 5}
	d.answer = r
	return r, nil
}

func TestWhisperUsesPermissionsAndConversationState(t *testing.T) {
	for _, tt := range []struct {
		name                                 string
		permissions                          []string
		incoming, outgoing, operator, closed bool
		queryError                           error
		want                                 error
	}{
		{name: "no identity-based peer restriction", permissions: []string{"message.initiate"}},
		{name: "one pending message", permissions: []string{"message.initiate"}, outgoing: true, want: model.ErrWhisperOnePending},
		{name: "reply needs permission", permissions: []string{"message.initiate"}, incoming: true, want: model.ErrNoPermission},
		{name: "reply-only user", permissions: []string{"message.reply"}, incoming: true},
		{name: "reply cannot initiate", permissions: []string{"message.reply"}, want: model.ErrNoPermission},
		{name: "revoked permissions", want: model.ErrNoPermission},
		{name: "operator respects pending limit", operator: true, outgoing: true, want: model.ErrWhisperOnePending},
		{name: "closed recipient", permissions: []string{"message.initiate"}, closed: true, want: model.ErrNoPermission},
		{name: "storage error fails closed", permissions: []string{"message.initiate"}, queryError: errors.New("offline"), want: model.ErrSendWhisperFailed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sender := permissionUser(1, tt.permissions...)
			sender.IsOperator = tt.operator
			receiver := permissionUser(2)
			if tt.closed {
				receiver.Status = ms.UserStatusClosed
			}
			ds := &permissionData{incoming: tt.incoming, outgoing: tt.outgoing, queryError: tt.queryError}
			s := &chatSrv{DaoServant: &base.DaoServant{Ds: ds}}
			got := s.canWhisper(sender, receiver)
			if (got == nil) != (tt.want == nil) || (got != nil && got != tt.want) {
				t.Fatalf("canWhisper = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepliesRespectHiddenParents(t *testing.T) {
	for _, status := range []ms.PostAuditT{ms.PostAuditPending, ms.PostAuditRejected} {
		if canReplyToComment(permissionUser(1, "comment.create"), 2, status) {
			t.Fatal("unrelated user can reply to hidden parent")
		}
		if got := canReplyToComment(permissionUser(2, "comment.create"), 2, status); got != (status == ms.PostAuditPending) {
			t.Fatal("author may reply only to pending parents")
		}
		if canReplyToComment(permissionUser(1, "comment.create", "content.review"), 2, status) {
			t.Fatal("unassigned reviewer can reply to hidden parent")
		}
		if got := canReplyToComment(permissionUser(1, "comment.create", "audit.view_all"), 2, status); got != (status == ms.PostAuditPending) {
			t.Fatal("review administrator may reply only to pending parents")
		}
	}
	if canReplyToComment(permissionUser(2), 2, ms.PostAuditApproved) {
		t.Fatal("ownership bypasses revoked comment permission")
	}
}

func TestCourseQuestionsAndAnswersAlwaysRequireReview(t *testing.T) {
	previousApp, previousAudit := conf.AppSetting, conf.AuditSetting
	conf.AppSetting = nil
	if err := json.Unmarshal([]byte(`{"MaxCommentCount":100}`), &conf.AppSetting); err != nil {
		t.Fatal(err)
	}
	conf.AuditSetting = &conf.AuditConf{Enabled: false}
	t.Cleanup(func() { conf.AppSetting, conf.AuditSetting = previousApp, previousAudit })
	user := permissionUser(1, "course.view", "comment.create", "content.publish_unreviewed")
	user.IsOperator = true
	ds := &permissionData{user: user, parent: &ms.CourseComment{Model: &ms.Model{ID: 4}, CourseID: 3, UserID: 2, AuditStatus: ms.PostAuditApproved}}
	s := &coursePrivSrv{DaoServant: &base.DaoServant{Ds: ds}}
	_, err := s.CreateCourseComment(&model.CreateCourseCommentReq{SimpleInfo: model.SimpleInfo{Uid: 1}, CourseID: 3, Contents: []*model.PostContentItem{{Content: "Question", Type: ms.ContentTypeText}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateCourseCommentReply(&model.CreateCourseCommentReplyReq{SimpleInfo: model.SimpleInfo{Uid: 1}, CommentID: 4, Content: "Answer"})
	if err != nil {
		t.Fatal(err)
	}
	if ds.question.AuditStatus != ms.PostAuditPending || ds.answer.AuditStatus != ms.PostAuditPending {
		t.Fatal("course Q&A bypassed mandatory moderation")
	}
	ds.user = permissionUser(1, "course.view", "comment.create")
	ds.parent.AuditStatus = ms.PostAuditRejected
	ds.answer = nil
	if _, err = s.CreateCourseCommentReply(&model.CreateCourseCommentReplyReq{SimpleInfo: model.SimpleInfo{Uid: 1}, CommentID: 4, Content: "Hidden answer"}); err != model.ErrNoPermission || ds.answer != nil {
		t.Fatalf("hidden-parent reply: err=%v, persisted=%v", err, ds.answer)
	}
}

type permissionStorage struct{ core.ObjectStorageService }

func (*permissionStorage) IsObjectExist(string) (bool, error) { return true, nil }
func (*permissionStorage) PersistObject(string) error         { return nil }
func (*permissionStorage) ObjectURL(key string) string        { return key }

func TestCourseOwnershipAndUploadPermissions(t *testing.T) {
	owner := permissionUser(1, "course.manage_own")
	if !canManageCourse(owner, 1) || canManageCourse(owner, 2) {
		t.Fatal("own-course permission escaped ownership")
	}
	if canUploadCourse(owner) || canUploadCourse(permissionUser(1, "course.upload")) {
		t.Fatal("course upload requires both capabilities")
	}
	if !canUploadCourse(permissionUser(1, "course.manage_own", "course.upload")) {
		t.Fatal("authorized teacher cannot upload")
	}
	ds := &permissionData{user: owner}
	s := &courseAdminSrv{DaoServant: &base.DaoServant{Ds: ds}, oss: &permissionStorage{}}
	_, err := s.CreateCourse(&model.CreateCourseReq{BaseInfo: model.BaseInfo{User: owner}, GroupID: 6, TeacherID: 99, Title: "Course"})
	if err != nil || ds.course == nil || ds.course.TeacherID != owner.ID {
		t.Fatalf("teacher must be forced to self: course=%v err=%v", ds.course, err)
	}
	if err := s.UpdateCourse(&model.UpdateCourseReq{BaseInfo: model.BaseInfo{User: owner}, ID: 3, TeacherID: 2}); err != model.ErrNoPermission {
		t.Fatalf("teacher reassignment must fail, got %v", err)
	}
	other := permissionUser(2, "course.manage_own")
	if err := s.DeleteCourse(&model.DeleteCourseReq{BaseInfo: model.BaseInfo{User: other}, ID: 3}); err != model.ErrNoPermission {
		t.Fatalf("cross-teacher delete must fail, got %v", err)
	}
	if _, err := s.CreateCourseGroup(&model.CourseGroupReq{BaseInfo: model.BaseInfo{User: owner}, Name: "Category"}); err != model.ErrNoPermission {
		t.Fatalf("teacher cannot manage global taxonomy, got %v", err)
	}
}
