package jinzhu

import (
	"errors"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *tweetManageSrv) CreatePostStar(postID, userID int64) (*ms.PostStar, error) {
	star := &ms.PostStar{PostID: postID, UserID: userID}
	err := setPostReaction(s.db, postID, userID, star, "upvote_count", true)
	return star, err
}
func (s *tweetManageSrv) DeletePostStar(star *ms.PostStar) error {
	return setPostReaction(s.db, star.PostID, star.UserID, star, "upvote_count", false)
}
func (s *tweetManageSrv) CreatePostCollection(postID, userID int64) (*ms.PostCollection, error) {
	collection := &ms.PostCollection{PostID: postID, UserID: userID}
	err := setPostReaction(s.db, postID, userID, collection, "collection_count", true)
	return collection, err
}
func (s *tweetManageSrv) DeletePostCollection(collection *ms.PostCollection) error {
	return setPostReaction(s.db, collection.PostID, collection.UserID, collection, "collection_count", false)
}

// Only the reaction column is written. A post snapshot from before moderation
// cannot restore its old audit state, visibility, content flags or other counts.
func setPostReaction(db *gorm.DB, postID, userID int64, model any, counter string, create bool) error {
	return db.Transaction(func(tx *gorm.DB) error {
		actor, err := lockIdentityPolicy(tx, authorStub(userID), authz.CommunityInteract)
		if err != nil {
			return err
		}
		var post ms.Post
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND is_del = 0", postID).First(&post).Error; err != nil {
			return err
		}
		if err := requirePostAccessTx(tx, actor, &post); err != nil {
			return err
		}
		query := tx.Where("post_id = ? AND user_id = ? AND is_del = 0", postID, userID)
		err = query.Omit("Post").Take(model).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if create && errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Omit("Post").Create(model).Error; err != nil {
				return err
			}
		} else if !create && err == nil {
			if err := tx.Omit("Post").Delete(model).Error; err != nil {
				return err
			}
		}
		var count int64
		counterModel := any(&ms.PostStar{})
		if counter == "collection_count" {
			counterModel = &ms.PostCollection{}
		}
		if err := tx.Model(counterModel).Where("post_id = ? AND is_del = 0", postID).Count(&count).Error; err != nil {
			return err
		}
		if err := tx.Model(&ms.Post{}).Where("id = ? AND is_del = 0", postID).Update(counter, count).Error; err != nil {
			return err
		}
		return refreshReviewedPostMetric(tx, postID)
	})
}

// The policy and post rows must already be locked. Recheck visibility against
// the committed parent, rather than a handler's earlier snapshot.
func requirePostAccessTx(tx *gorm.DB, actor *ms.User, post *ms.Post) error {
	if !actor.HasPermission(authz.PostView) {
		return authz.ErrDenied
	}
	if post.UserID == actor.ID {
		return nil
	}
	if post.AuditStatus != ms.PostAuditApproved {
		if actor.HasPermission(authz.AuditViewAll) {
			return nil
		}
		var assigned int64
		if !actor.HasPermission(authz.ContentReview) {
			return authz.ErrDenied
		}
		if err := tx.Model(&ms.ReviewTask{}).Where("kind = ? AND target_id = ? AND assignee_id = ?", ms.ReviewPost, post.ID, actor.ID).Count(&assigned).Error; err != nil {
			return err
		}
		if assigned > 0 {
			return nil
		}
		return authz.ErrDenied
	}
	if actor.HasPermission(authz.ContentViewPrivate) || post.Visibility == ms.PostVisitPublic {
		return nil
	}
	if post.Visibility == ms.PostVisitFollowing {
		var follows int64
		if err := tx.Model(&dbr.Following{}).Where("user_id = ? AND follow_id = ?", actor.ID, post.UserID).Count(&follows).Error; err != nil {
			return err
		}
		if follows > 0 {
			return nil
		}
	}
	return authz.ErrDenied
}
