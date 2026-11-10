package web

import (
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/application/content"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	model "github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
)

type reactionTargetData struct {
	core.DataService
	post    *ms.Post
	user    *ms.User
	comment *ms.Comment
	reply   *ms.CommentReply
}

func (d *reactionTargetData) GetUserByID(int64) (*ms.User, error)       { return d.user, nil }
func (d *reactionTargetData) GetPostByID(int64) (*ms.Post, error)       { return d.post, nil }
func (d *reactionTargetData) GetCommentByID(int64) (*ms.Comment, error) { return d.comment, nil }
func (d *reactionTargetData) GetCommentReplyByID(int64) (*ms.CommentReply, error) {
	return d.reply, nil
}

func TestReactionTargetsRejectMismatchedAndHiddenParents(t *testing.T) {
	d := &reactionTargetData{
		user:    &ms.User{Model: &ms.Model{ID: 2}, Status: ms.UserStatusNormal, Permissions: []string{"post.view", "community.interact"}},
		post:    &ms.Post{Model: &ms.Model{ID: 10}, UserID: 1, Visibility: ms.PostVisitPublic, AuditStatus: ms.PostAuditApproved},
		comment: &ms.Comment{Model: &ms.Model{ID: 20}, PostID: 10},
		reply:   &ms.CommentReply{Model: &ms.Model{ID: 30}, CommentID: 20},
	}
	s := &privSrv{DaoServant: &base.DaoServant{Ds: d, Views: content.New(d)}}
	if err := s.checkReplyThumbsTarget(10, 20, 30, 2); err != nil {
		t.Fatalf("valid target: %v", err)
	}
	for _, tc := range []struct {
		name   string
		change func()
	}{
		{"wrong post", func() { d.comment.PostID = 11 }},
		{"wrong comment", func() { d.comment.PostID = 10; d.reply.CommentID = 21 }},
		{"private post", func() { d.reply.CommentID = 20; d.post.Visibility = ms.PostVisitPrivate }},
		{"pending post", func() { d.post.Visibility = ms.PostVisitPublic; d.post.AuditStatus = ms.PostAuditPending }},
		{"no interaction permission", func() { d.post.AuditStatus = ms.PostAuditApproved; d.user.Permissions = []string{"post.view"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.change()
			if err := s.checkReplyThumbsTarget(10, 20, 30, 2); err != model.ErrNoPermission {
				t.Fatalf("got %v, want permission denial", err)
			}
		})
	}
}
