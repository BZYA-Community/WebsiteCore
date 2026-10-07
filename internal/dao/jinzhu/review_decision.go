package jinzhu

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type reviewSubject struct {
	AuthorID int64
	ParentID int64
	Pending  bool
	Status   int
	Snapshot string
	OldValue string
	Post     *ms.Post
	Comment  *ms.Comment
	Reply    *ms.CommentReply
	Question *ms.CourseComment
	Answer   *ms.CourseCommentReply
	User     *ms.User
}

func loadReviewSubject(tx *gorm.DB, kind string, id int64) (*reviewSubject, error) {
	result := &reviewSubject{}
	row := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND is_del = 0", id)
	switch kind {
	case ms.ReviewPost:
		post := &ms.Post{}
		if err := row.First(post).Error; err != nil {
			return nil, err
		}
		result.Post, result.AuthorID, result.ParentID, result.Status = post, post.UserID, post.ID, int(post.AuditStatus)
		result.Pending = post.AuditStatus == ms.PostAuditPending && post.Visibility != ms.PostVisitPrivate
	case ms.ReviewComment:
		comment := &ms.Comment{}
		if err := row.First(comment).Error; err != nil {
			return nil, err
		}
		var parent ms.Post
		if err := tx.Where("id = ? AND is_del = 0", comment.PostID).First(&parent).Error; err != nil {
			return nil, err
		}
		result.Comment, result.AuthorID, result.ParentID, result.Status = comment, comment.UserID, comment.PostID, int(comment.AuditStatus)
		result.Pending = comment.AuditStatus == ms.PostAuditPending
	case ms.ReviewReply:
		reply := &ms.CommentReply{}
		if err := row.First(reply).Error; err != nil {
			return nil, err
		}
		var comment ms.Comment
		if err := tx.Where("id = ? AND is_del = 0", reply.CommentID).First(&comment).Error; err != nil {
			return nil, err
		}
		var parent ms.Post
		if err := tx.Where("id = ? AND is_del = 0", comment.PostID).First(&parent).Error; err != nil {
			return nil, err
		}
		result.Reply, result.AuthorID, result.ParentID, result.Status = reply, reply.UserID, comment.PostID, int(reply.AuditStatus)
		result.Pending = reply.AuditStatus == ms.PostAuditPending
	case ms.ReviewCourseQuestion:
		question := &ms.CourseComment{}
		if err := row.First(question).Error; err != nil {
			return nil, err
		}
		var course ms.Course
		if err := tx.Where("id = ? AND is_del = 0", question.CourseID).First(&course).Error; err != nil {
			return nil, err
		}
		result.Question, result.AuthorID, result.ParentID, result.Status = question, question.UserID, question.CourseID, int(question.AuditStatus)
		result.Pending = question.AuditStatus == ms.PostAuditPending
	case ms.ReviewCourseAnswer:
		answer := &ms.CourseCommentReply{}
		if err := row.First(answer).Error; err != nil {
			return nil, err
		}
		var question ms.CourseComment
		if err := tx.Where("id = ? AND is_del = 0", answer.CommentID).First(&question).Error; err != nil {
			return nil, err
		}
		var course ms.Course
		if err := tx.Where("id = ? AND is_del = 0", question.CourseID).First(&course).Error; err != nil {
			return nil, err
		}
		result.Answer, result.AuthorID, result.ParentID, result.Status = answer, answer.UserID, question.CourseID, int(answer.AuditStatus)
		result.Pending = answer.AuditStatus == ms.PostAuditPending
	case ms.ReviewNickname, ms.ReviewAvatar:
		user := &ms.User{}
		if err := row.First(user).Error; err != nil {
			return nil, err
		}
		result.User, result.AuthorID = user, user.ID
		if kind == ms.ReviewNickname {
			result.Snapshot, result.OldValue = user.PendingNickname, user.Nickname
		} else {
			result.Snapshot, result.OldValue = user.PendingAvatar, user.Avatar
		}
		result.Pending = result.Snapshot != ""
	default:
		return nil, authz.ErrInvalid
	}
	return result, nil
}

func (s *reviewSrv) DecideReview(actor *ms.User, taskID, revision int64, action, reason string) (*core.ReviewDecision, error) {
	reason = strings.TrimSpace(reason)
	if taskID <= 0 || revision <= 0 || (action != "approve" && action != "reject") ||
		(action == "reject" && reason == "") || utf8.RuneCountInString(reason) > 255 {
		return nil, authz.ErrInvalid
	}
	var result *core.ReviewDecision
	err := s.db.Transaction(func(tx *gorm.DB) error {
		fresh, err := lockIdentityPolicy(tx, actor, authz.ContentReview)
		if err != nil {
			return err
		}
		var task ms.ReviewTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", taskID).First(&task).Error; err != nil {
			return err
		}
		now := time.Now().Unix()
		if task.State != ms.ReviewPending || task.Revision != revision {
			return core.ErrReviewStale
		}
		if !fresh.IsOperator && (task.AssigneeID != fresh.ID || task.AuthorID == fresh.ID || task.DeadlineOn <= now) {
			return authz.ErrDenied
		}
		subject, err := loadReviewSubject(tx, task.Kind, task.TargetID)
		if err != nil {
			return err
		}
		if !subject.Pending || subject.Snapshot != task.Snapshot {
			return core.ErrReviewStale
		}
		newStatus := int(ms.PostAuditApproved)
		if action == "reject" {
			newStatus = int(ms.PostAuditRejected)
		}
		result = &core.ReviewDecision{Task: &task, OldStatus: subject.Status, NewStatus: newStatus,
			ParentID: subject.ParentID, OldValue: subject.OldValue, NewValue: subject.Snapshot}
		if err := applyReviewDecision(tx, &task, subject, newStatus, now); err != nil {
			return err
		}
		task.State, task.CompletedOn, task.ModifiedOn = ms.ReviewCompleted, now, now
		if err := tx.Save(&task).Error; err != nil {
			return err
		}
		if fresh.IsOperator && task.AssigneeID != fresh.ID {
			if err := reviewEvent(tx, &task, "operator_override", fresh.ID, task.AssigneeID, fresh.ID, reason, now); err != nil {
				return err
			}
		}
		if err := reviewEvent(tx, &task, action, fresh.ID, task.AssigneeID, fresh.ID, reason, now); err != nil {
			return err
		}
		logAction := action
		if task.Kind != ms.ReviewPost {
			logAction = task.Kind + "_" + action
		}
		return tx.Create(&ms.AuditLog{PostID: subject.ParentID, OperatorID: fresh.ID, Action: logAction,
			OldStatus: uint8(subject.Status), NewStatus: uint8(newStatus), Reason: reason}).Error
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func applyReviewDecision(tx *gorm.DB, task *ms.ReviewTask, subject *reviewSubject, status int, now int64) error {
	changes := map[string]any{"audit_status": status, "modified_on": now}
	approved := status == int(ms.PostAuditApproved)
	var content any
	switch task.Kind {
	case ms.ReviewPost:
		content = &ms.Post{}
		if !approved {
			changes["visibility"] = ms.PostVisitPrivate
		}
		if approved && subject.Post.Visibility != ms.PostVisitPrivate {
			seen := map[string]bool{}
			for _, tag := range strings.Split(subject.Post.Tags, ",") {
				tag = strings.TrimSpace(tag)
				if tag == "" || seen[tag] {
					continue
				}
				seen[tag] = true
				entry := &dbr.Tag{Model: &dbr.Model{}, UserID: subject.AuthorID, Tag: tag, QuoteNum: 1}
				if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tag"}}, DoUpdates: clause.Assignments(map[string]any{
					"quote_num": gorm.Expr("? + 1", clause.Column{Table: clause.CurrentTable, Name: "quote_num"}), "modified_on": now,
				})}).Create(entry).Error; err != nil {
					return err
				}
			}
		}
	case ms.ReviewComment:
		content = &ms.Comment{}
		if approved {
			if err := incrementReviewedPostComments(tx, subject.ParentID, subject.Comment.CreatedOn); err != nil {
				return err
			}
		}
	case ms.ReviewReply:
		content = &ms.CommentReply{}
		if approved {
			if err := tx.Model(&ms.Comment{}).Where("id = ? AND is_del = 0", subject.Reply.CommentID).Update("reply_count", gorm.Expr("reply_count + 1")).Error; err != nil {
				return err
			}
			if err := incrementReviewedPostComments(tx, subject.ParentID, subject.Reply.CreatedOn); err != nil {
				return err
			}
		}
	case ms.ReviewCourseQuestion:
		content = &ms.CourseComment{}
		if approved {
			if err := incrementReviewedCourseComments(tx, subject.ParentID); err != nil {
				return err
			}
		}
	case ms.ReviewCourseAnswer:
		content = &ms.CourseCommentReply{}
		if approved {
			if err := tx.Model(&ms.CourseComment{}).Where("id = ? AND is_del = 0", subject.Answer.CommentID).Update("reply_count", gorm.Expr("reply_count + 1")).Error; err != nil {
				return err
			}
			if err := incrementReviewedCourseComments(tx, subject.ParentID); err != nil {
				return err
			}
		}
	case ms.ReviewNickname, ms.ReviewAvatar:
		field, pending := "nickname", "pending_nickname"
		if task.Kind == ms.ReviewAvatar {
			field, pending = "avatar", "pending_avatar"
		}
		changes = map[string]any{pending: "", "modified_on": now}
		if approved {
			changes[field] = subject.Snapshot
		}
		content = &ms.User{}
	default:
		return authz.ErrInvalid
	}
	result := tx.Model(content).Where("id = ? AND is_del = 0", task.TargetID).Updates(changes)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return core.ErrReviewStale
	}
	return nil
}

func incrementReviewedPostComments(tx *gorm.DB, id, createdOn int64) error {
	if err := tx.Model(&ms.Post{}).Where("id = ? AND is_del = 0", id).Updates(map[string]any{
		"comment_count":     gorm.Expr("comment_count + 1"),
		"latest_replied_on": gorm.Expr("GREATEST(latest_replied_on, ?)", createdOn),
	}).Error; err != nil {
		return err
	}
	return refreshReviewedPostMetric(tx, id)
}

func refreshReviewedPostMetric(tx *gorm.DB, id int64) error {
	var post ms.Post
	if err := tx.Where("id = ? AND is_del = 0", id).First(&post).Error; err != nil {
		return err
	}
	return newTweetMetricServentA(tx).UpdateTweetMetric(&cs.TweetMetric{PostId: post.ID,
		CommentCount: post.CommentCount, UpvoteCount: post.UpvoteCount,
		CollectionCount: post.CollectionCount, ShareCount: post.ShareCount})
}

func incrementReviewedCourseComments(tx *gorm.DB, id int64) error {
	return tx.Model(&ms.Course{}).Where("id = ? AND is_del = 0", id).Update("comment_count", gorm.Expr("comment_count + 1")).Error
}
