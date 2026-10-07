package jinzhu

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Deletion uses the same lock order as review, so a stale handler snapshot
// cannot miss a just-approved reply or overwrite a moderation decision.
func (s *commentManageSrv) DeleteComment(comment *ms.Comment) error {
	return deleteReviewedComment(s.db, ms.ReviewComment, comment.ID)
}

func (s *commentManageSrv) DeleteCommentReply(reply *ms.CommentReply) error {
	return deleteReviewedComment(s.db, ms.ReviewReply, reply.ID)
}

func (s *courseManageSrv) DeleteCourseComment(question *ms.CourseComment) error {
	return deleteReviewedComment(s.db, ms.ReviewCourseQuestion, question.ID)
}

func (s *courseManageSrv) DeleteCourseCommentReply(answer *ms.CourseCommentReply) error {
	return deleteReviewedComment(s.db, ms.ReviewCourseAnswer, answer.ID)
}

func deleteReviewedComment(db *gorm.DB, kind string, id int64) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := reviewPolicyLock(tx); err != nil {
			return err
		}
		subject, err := loadReviewSubject(tx, kind, id)
		if err != nil {
			return err
		}
		removed := int64(0)
		if subject.Status == int(ms.PostAuditApproved) {
			removed++
		}
		var model any
		switch kind {
		case ms.ReviewComment:
			model = &ms.Comment{}
			var replies []*ms.CommentReply
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("comment_id = ? AND is_del = 0", id).Find(&replies).Error; err != nil {
				return err
			}
			ids := make([]int64, 0, len(replies))
			for _, reply := range replies {
				ids = append(ids, reply.ID)
				if reply.AuditStatus == ms.PostAuditApproved {
					removed++
				}
			}
			if err := cancelTargetReviewsTx(tx, ms.ReviewReply, ids); err != nil {
				return err
			}
			if err := tx.Where("comment_id = ?", id).Delete(&ms.CommentReply{}).Error; err != nil {
				return err
			}
			if err := tx.Where("comment_id = ?", id).Delete(&dbr.TweetCommentThumbs{}).Error; err != nil {
				return err
			}
		case ms.ReviewReply:
			model = &ms.CommentReply{}
			if removed > 0 {
				if err := tx.Model(&ms.Comment{}).Where("id = ?", subject.Reply.CommentID).Update("reply_count", gorm.Expr("GREATEST(reply_count - 1, 0)")).Error; err != nil {
					return err
				}
			}
			if err := tx.Where("reply_id = ?", id).Delete(&dbr.TweetCommentThumbs{}).Error; err != nil {
				return err
			}
		case ms.ReviewCourseQuestion:
			model = &ms.CourseComment{}
			var answers []*ms.CourseCommentReply
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("comment_id = ? AND is_del = 0", id).Find(&answers).Error; err != nil {
				return err
			}
			ids := make([]int64, 0, len(answers))
			for _, answer := range answers {
				ids = append(ids, answer.ID)
				if answer.AuditStatus == ms.PostAuditApproved {
					removed++
				}
			}
			if err := cancelTargetReviewsTx(tx, ms.ReviewCourseAnswer, ids); err != nil {
				return err
			}
			if err := tx.Where("comment_id = ?", id).Delete(&ms.CourseCommentReply{}).Error; err != nil {
				return err
			}
		case ms.ReviewCourseAnswer:
			model = &ms.CourseCommentReply{}
			if removed > 0 {
				if err := tx.Model(&ms.CourseComment{}).Where("id = ?", subject.Answer.CommentID).Update("reply_count", gorm.Expr("GREATEST(reply_count - 1, 0)")).Error; err != nil {
					return err
				}
			}
		}
		if err := cancelTargetReviewsTx(tx, kind, []int64{id}); err != nil {
			return err
		}
		if err := tx.Where("id = ?", id).Delete(model).Error; err != nil {
			return err
		}
		if removed == 0 {
			return nil
		}
		parent := any(&ms.Post{})
		if kind == ms.ReviewCourseQuestion || kind == ms.ReviewCourseAnswer {
			parent = &ms.Course{}
		}
		if err := tx.Model(parent).Where("id = ? AND is_del = 0", subject.ParentID).Update("comment_count", gorm.Expr("GREATEST(comment_count - ?, 0)", removed)).Error; err != nil {
			return err
		}
		if kind == ms.ReviewComment || kind == ms.ReviewReply {
			return refreshReviewedPostMetric(tx, subject.ParentID)
		}
		return nil
	})
}
