package jinzhu

import (
	"strings"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func reviewRequired(user *ms.User) bool {
	return conf.AuditSetting != nil && conf.AuditSetting.Enabled && !user.HasPermission(authz.PublishUnreviewed)
}

func authorStub(id int64) *ms.User { return &ms.User{Model: &ms.Model{ID: id}} }

func (s *reviewSrv) CreatePostWithContents(post *ms.Post, contents []*ms.PostContent) (*ms.Post, error) {
	if post == nil {
		return nil, authz.ErrInvalid
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		user, err := lockIdentityPolicy(tx, authorStub(post.UserID), authz.PostCreate)
		if err != nil {
			return err
		}
		post.AuditStatus = ms.PostAuditApproved
		if reviewRequired(user) && post.Visibility != ms.PostVisitPrivate {
			post.AuditStatus = ms.PostAuditPending
		}
		post.LatestRepliedOn = time.Now().Unix()
		if _, err := post.Create(tx); err != nil {
			return err
		}
		if _, err := (&dbr.PostMetric{PostId: post.ID}).Create(tx); err != nil {
			return err
		}
		for _, content := range contents {
			content.PostID, content.UserID = post.ID, post.UserID
			if _, err := content.Create(tx); err != nil {
				return err
			}
		}
		if post.AuditStatus == ms.PostAuditApproved && post.Visibility != ms.PostVisitPrivate {
			if _, err := createTags(tx, post.UserID, reviewTags(post.Tags)); err != nil {
				return err
			}
		}
		return enqueueReviewTx(tx, ms.ReviewPost, post.ID, conf.ReviewDeadline())
	})
	return post, err
}

func (s *reviewSrv) CreateCommentWithContents(comment *ms.Comment, contents []*ms.CommentContent) (*ms.Comment, error) {
	if comment == nil {
		return nil, authz.ErrInvalid
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		user, err := lockIdentityPolicy(tx, authorStub(comment.UserID), authz.CommentCreate)
		if err != nil {
			return err
		}
		var post ms.Post
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND is_del = 0", comment.PostID).First(&post).Error; err != nil {
			return err
		}
		if post.IsLock > 0 {
			return authz.ErrDenied
		}
		if err := requirePostAccessTx(tx, user, &post); err != nil {
			return err
		}
		comment.AuditStatus = ms.PostAuditApproved
		if reviewRequired(user) && post.Visibility != ms.PostVisitPrivate {
			comment.AuditStatus = ms.PostAuditPending
		}
		if _, err := comment.Create(tx); err != nil {
			return err
		}
		for _, content := range contents {
			content.CommentID, content.UserID = comment.ID, comment.UserID
			if _, err := content.Create(tx); err != nil {
				return err
			}
		}
		if comment.AuditStatus == ms.PostAuditApproved {
			if err := incrementReviewedPostComments(tx, post.ID, comment.CreatedOn); err != nil {
				return err
			}
		}
		return enqueueReviewTx(tx, ms.ReviewComment, comment.ID, conf.ReviewDeadline())
	})
	return comment, err
}

func (s *commentManageSrv) CreateCommentReply(reply *ms.CommentReply) (*ms.CommentReply, error) {
	if reply == nil {
		return nil, authz.ErrInvalid
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		user, err := lockIdentityPolicy(tx, authorStub(reply.UserID), authz.CommentCreate)
		if err != nil {
			return err
		}
		var comment ms.Comment
		if err := tx.Where("id = ? AND is_del = 0", reply.CommentID).First(&comment).Error; err != nil {
			return err
		}
		var post ms.Post
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND is_del = 0", comment.PostID).First(&post).Error; err != nil {
			return err
		}
		if post.IsLock > 0 {
			return authz.ErrDenied
		}
		if err := requirePostAccessTx(tx, user, &post); err != nil {
			return err
		}
		if comment.AuditStatus != ms.PostAuditApproved && comment.UserID != user.ID && !user.HasPermission(authz.AuditViewAll) {
			return authz.ErrDenied
		}
		reply.AuditStatus = ms.PostAuditApproved
		if reviewRequired(user) && post.Visibility != ms.PostVisitPrivate {
			reply.AuditStatus = ms.PostAuditPending
		}
		if _, err := reply.Create(tx); err != nil {
			return err
		}
		if reply.AuditStatus == ms.PostAuditApproved {
			if err := tx.Model(&ms.Comment{}).Where("id = ?", reply.CommentID).Update("reply_count", gorm.Expr("reply_count + 1")).Error; err != nil {
				return err
			}
			if err := incrementReviewedPostComments(tx, post.ID, reply.CreatedOn); err != nil {
				return err
			}
		}
		return enqueueReviewTx(tx, ms.ReviewReply, reply.ID, conf.ReviewDeadline())
	})
	return reply, err
}

func (s *reviewSrv) CreateCourseQuestionWithContents(question *ms.CourseComment, contents []*ms.CourseCommentContent) (*ms.CourseComment, error) {
	if question == nil {
		return nil, authz.ErrInvalid
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		user, err := lockIdentityPolicy(tx, authorStub(question.UserID), authz.CommentCreate)
		if err != nil {
			return err
		}
		if !user.HasPermission(authz.CourseView) {
			return authz.ErrDenied
		}
		var course ms.Course
		if err := tx.Where("id = ? AND is_del = 0", question.CourseID).First(&course).Error; err != nil {
			return err
		}
		question.AuditStatus = ms.PostAuditPending
		if _, err := question.Create(tx); err != nil {
			return err
		}
		for _, content := range contents {
			content.CommentID, content.UserID = question.ID, question.UserID
			if _, err := content.Create(tx); err != nil {
				return err
			}
		}
		return enqueueReviewTx(tx, ms.ReviewCourseQuestion, question.ID, conf.ReviewDeadline())
	})
	return question, err
}

func (s *courseManageSrv) CreateCourseCommentReply(reply *ms.CourseCommentReply) (*ms.CourseCommentReply, error) {
	if reply == nil {
		return nil, authz.ErrInvalid
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		user, err := lockIdentityPolicy(tx, authorStub(reply.UserID), authz.CommentCreate)
		if err != nil {
			return err
		}
		if !user.HasPermission(authz.CourseView) {
			return authz.ErrDenied
		}
		var question ms.CourseComment
		if err := tx.Where("id = ? AND is_del = 0", reply.CommentID).First(&question).Error; err != nil {
			return err
		}
		if question.AuditStatus != ms.PostAuditApproved && question.UserID != user.ID && !user.HasPermission(authz.AuditViewAll) {
			return authz.ErrDenied
		}
		reply.AuditStatus = ms.PostAuditPending
		if _, err := reply.Create(tx); err != nil {
			return err
		}
		return enqueueReviewTx(tx, ms.ReviewCourseAnswer, reply.ID, conf.ReviewDeadline())
	})
	return reply, err
}

func (s *reviewSrv) SubmitProfileReview(user *ms.User, kind, value string) (*core.ProfileSubmission, error) {
	if value == "" || (kind != ms.ReviewNickname && kind != ms.ReviewAvatar) {
		return nil, authz.ErrInvalid
	}
	result := &core.ProfileSubmission{}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		fresh, err := lockIdentityPolicy(tx, user, authz.ProfileEdit)
		if err != nil {
			return err
		}
		column, visible := "pending_nickname", "nickname"
		result.Previous = fresh.Nickname
		if kind == ms.ReviewAvatar {
			column, visible = "pending_avatar", "avatar"
			result.Previous = fresh.Avatar
		}
		result.Pending = reviewRequired(fresh)
		changes := map[string]any{column: value}
		if !result.Pending {
			changes = map[string]any{column: "", visible: value}
		}
		if err := tx.Model(&ms.User{}).Where("id = ? AND is_del = 0", fresh.ID).Updates(changes).Error; err != nil {
			return err
		}
		return enqueueReviewTx(tx, kind, fresh.ID, conf.ReviewDeadline())
	})
	return result, err
}

func (s *reviewSrv) SetPostVisibilityForReview(actor *ms.User, postID int64, visibility ms.PostVisibleT) (*ms.Post, error) {
	if visibility != ms.PostVisitPublic && visibility != ms.PostVisitFollowing && visibility != ms.PostVisitPrivate {
		return nil, authz.ErrInvalid
	}
	var post ms.Post
	err := s.db.Transaction(func(tx *gorm.DB) error {
		fresh, err := lockIdentityPolicy(tx, actor, authz.PostCreate)
		if err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND is_del = 0", postID).First(&post).Error; err != nil {
			return err
		}
		if post.UserID != fresh.ID && !fresh.HasPermission(authz.ContentManage) {
			return authz.ErrDenied
		}
		oldVisibility, oldStatus := post.Visibility, post.AuditStatus
		post.Visibility = visibility
		if visibility == ms.PostVisitPrivate {
			post.IsTop = 0
		}
		if oldVisibility == ms.PostVisitPrivate && visibility != ms.PostVisitPrivate && reviewRequired(fresh) {
			post.AuditStatus = ms.PostAuditPending
		}
		if err := tx.Model(&ms.Post{}).Where("id = ?", post.ID).Updates(map[string]any{
			"visibility": post.Visibility, "is_top": post.IsTop, "audit_status": post.AuditStatus,
		}).Error; err != nil {
			return err
		}
		tags := reviewTags(post.Tags)
		if oldStatus == ms.PostAuditApproved && oldVisibility != ms.PostVisitPrivate && (visibility == ms.PostVisitPrivate || post.AuditStatus != ms.PostAuditApproved) {
			if err := deleteTags(tx, tags); err != nil {
				return err
			}
		}
		if post.AuditStatus == ms.PostAuditApproved && oldVisibility == ms.PostVisitPrivate && visibility != ms.PostVisitPrivate {
			if _, err := createTags(tx, post.UserID, tags); err != nil {
				return err
			}
		}
		return enqueueReviewTx(tx, ms.ReviewPost, post.ID, conf.ReviewDeadline())
	})
	return &post, err
}

func reviewTags(encoded string) []string {
	tags := []string{}
	seen := map[string]bool{}
	for _, tag := range strings.Split(encoded, ",") {
		tag = strings.TrimSpace(tag)
		if tag != "" && !seen[tag] {
			tags = append(tags, tag)
			seen[tag] = true
		}
	}
	return tags
}
