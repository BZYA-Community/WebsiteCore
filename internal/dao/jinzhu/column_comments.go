package jinzhu

import (
	"strings"
	"unicode/utf8"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func columnCommentScope(db *gorm.DB, actor *ms.User) *gorm.DB {
	if actor != nil && actor.HasPermission(authz.AuditViewAll) {
		return db
	}
	if actor != nil && actor.ID > 0 {
		return db.Where("(audit_status=? OR user_id=?)", ms.PostAuditApproved, actor.ID)
	}
	return db.Where("audit_status=?", ms.PostAuditApproved)
}

func columnCommentVisible(actor *ms.User, c *ms.ColumnComment) bool {
	return c.AuditStatus == ms.PostAuditApproved || (actor != nil && (actor.ID == c.UserID || actor.HasPermission(authz.AuditViewAll)))
}

func (s *columnSrv) ColumnComments(actor *ms.User, id int64, style string, offset, limit int) ([]*ms.ColumnComment, int64, error) {
	row, err := loadColumn(s.db, id)
	if err != nil {
		return nil, 0, err
	}
	if !columnPublic(row) || row.Kind != ms.ColumnArticle {
		return nil, 0, gorm.ErrRecordNotFound
	}
	if offset < 0 || limit < 1 || limit > 100 {
		return nil, 0, authz.ErrInvalid
	}
	db := columnCommentScope(s.db.Model(&ms.ColumnComment{}).Where("column_id=? AND parent_id=0 AND is_del=0", id), actor)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	order := "featured DESC, id ASC"
	if style == "newest" {
		order = "featured DESC, id DESC"
	}
	if style == "hots" {
		table := s.db.NamingStrategy.TableName("column_comment")
		order = "featured DESC, ((SELECT COUNT(*) FROM " + table + " r WHERE r.parent_id=" + table + ".id AND r.is_del=0 AND r.audit_status=1)*2+likes*4-dislikes) DESC, id DESC"
	}
	comments := []*ms.ColumnComment{}
	if err := db.Order(order).Offset(offset).Limit(limit).Find(&comments).Error; err != nil {
		return nil, 0, err
	}
	ids := []int64{}
	byID := map[int64]*ms.ColumnComment{}
	for _, c := range comments {
		ids = append(ids, c.ID)
		byID[c.ID] = c
		c.Replies = []*ms.ColumnComment{}
	}
	replies := []*ms.ColumnComment{}
	if len(ids) > 0 {
		if err := columnCommentScope(s.db.Where("column_id=? AND parent_id IN ? AND is_del=0", id, ids), actor).Order("id ASC").Find(&replies).Error; err != nil {
			return nil, 0, err
		}
	}
	all := append(append([]*ms.ColumnComment{}, comments...), replies...)
	for _, c := range all {
		c.User = columnUser(s.db, c.UserID)
		if c.AtUserID > 0 {
			c.AtUser = columnUser(s.db, c.AtUserID)
		}
		if actor != nil && actor.ID > 0 {
			var r ms.ColumnReaction
			if s.db.Where("comment_id=? AND user_id=?", c.ID, actor.ID).First(&r).Error == nil {
				c.Reaction = r.Value
			}
		}
		if c.ParentID > 0 {
			if parent := byID[c.ParentID]; parent != nil {
				parent.Replies = append(parent.Replies, c)
			}
		}
	}
	return comments, total, nil
}

func validateColumnComment(c *ms.ColumnComment) error {
	if c == nil || c.ColumnID <= 0 || c.ParentID < 0 || strings.TrimSpace(c.Content) == "" || utf8.RuneCountInString(c.Content) > 300 || len(c.Mentions) > 100 {
		return authz.ErrInvalid
	}
	if c.Contents == nil {
		c.Contents = []*ms.ColumnContent{}
	}
	if c.Mentions == nil {
		c.Mentions = []string{}
	}
	images := 0
	for _, item := range c.Contents {
		if item == nil {
			return authz.ErrInvalid
		}
		if item.Type == ms.ContentTypeImage {
			images++
		} else if item.Type != ms.ContentTypeText {
			return authz.ErrInvalid
		}
	}
	if images > 9 || (c.ParentID > 0 && images > 0) {
		return authz.ErrInvalid
	}
	return nil
}

func (s *columnSrv) CreateColumnComment(actor *ms.User, input *ms.ColumnComment) (*ms.ColumnComment, error) {
	if err := validateColumnComment(input); err != nil {
		return nil, err
	}
	c := *input
	c.Model = &ms.Model{}
	c.Replies = nil
	c.User = nil
	c.Likes, c.Dislikes = 0, 0
	c.Featured = false
	err := columnTransaction(s.db, func(tx *gorm.DB) error {
		fresh, err := lockIdentityPolicy(tx, actor, authz.CommentCreate)
		if err != nil {
			return err
		}
		row, err := loadColumn(tx, c.ColumnID)
		if err != nil {
			return err
		}
		if !columnPublic(row) || row.Kind != ms.ColumnArticle || row.Locked {
			return authz.ErrDenied
		}
		if err := checkColumnCommentLimitTx(tx, row.ID); err != nil {
			return err
		}
		if c.ParentID > 0 {
			var parent ms.ColumnComment
			if err := tx.Where("id=? AND column_id=? AND parent_id=0 AND is_del=0", c.ParentID, row.ID).First(&parent).Error; err != nil {
				return err
			}
			if parent.AuditStatus == ms.PostAuditRejected || parent.AuditStatus == ms.ColumnWithdrawn || (parent.AuditStatus == ms.PostAuditPending && parent.UserID != fresh.ID && !fresh.HasPermission(authz.AuditViewAll)) {
				return authz.ErrDenied
			}
		}
		if c.AtUserID == fresh.ID {
			c.AtUserID = 0
		}
		if c.AtUserID > 0 {
			if _, err := (&ms.User{Model: &ms.Model{ID: c.AtUserID}}).Get(tx); err != nil {
				c.AtUserID = 0
			}
		}
		if err := checkColumnMediaTx(tx, fresh, nil, c.Contents); err != nil {
			return err
		}
		c.UserID = fresh.ID
		c.AuditStatus = ms.PostAuditApproved
		if reviewRequired(fresh) {
			c.AuditStatus = ms.PostAuditPending
		}
		if err := tx.Create(&c).Error; err != nil {
			return err
		}
		if c.AuditStatus == ms.PostAuditApproved {
			if err := notifyColumnCommentTx(tx, row, &c); err != nil {
				return err
			}
		}
		return enqueueReviewTx(tx, ms.ReviewColumnComment, c.ID, conf.ReviewDeadline())
	})
	return &c, err
}

// All callers hold the shared policy lock, serializing creation and approval.
// Count approved roots and replies, including replies awaiting a public parent.
func checkColumnCommentLimitTx(tx *gorm.DB, columnID int64) error {
	var count int64
	if err := tx.Model(&ms.ColumnComment{}).Where("column_id=? AND audit_status=? AND is_del=0", columnID, ms.PostAuditApproved).Count(&count).Error; err != nil {
		return err
	}
	if conf.AppSetting != nil && count >= conf.AppSetting.MaxCommentCount {
		return authz.ErrInvalid
	}
	return nil
}

func notifyColumnCommentTx(tx *gorm.DB, row *ms.Column, c *ms.ColumnComment) error {
	receivers := map[int64]bool{row.UserID: true}
	if c.AtUserID > 0 {
		receivers[c.AtUserID] = true
	}
	if c.ParentID > 0 {
		var parent ms.ColumnComment
		if err := tx.First(&parent, c.ParentID).Error; err != nil {
			return err
		}
		// An approved reply is still private until its root is approved. Root
		// approval delivers these deferred notifications in the same transaction.
		if parent.AuditStatus != ms.PostAuditApproved {
			return nil
		}
		receivers[parent.UserID] = true
	}
	for _, name := range c.Mentions {
		var user ms.User
		if tx.Where("username=? AND is_del=0", name).First(&user).Error == nil {
			receivers[user.ID] = true
		}
	}
	for id := range receivers {
		if err := columnMessage(tx, c.UserID, id, row.ID, c.ID, "专栏中有新的评论或回复："+row.Title); err != nil {
			return err
		}
	}
	return nil
}

func (s *columnSrv) ColumnCommentAction(actor *ms.User, id int64, action string) error {
	return columnTransaction(s.db, func(tx *gorm.DB) error {
		fresh, err := lockColumnActor(tx, actor)
		if err != nil {
			return err
		}
		var c ms.ColumnComment
		if err := tx.Where("id=? AND is_del=0", id).First(&c).Error; err != nil {
			return err
		}
		row, err := loadColumn(tx, c.ColumnID)
		if err != nil {
			return err
		}
		manager := fresh.HasPermission(authz.ContentManage)
		if action == "delete" {
			if c.UserID != fresh.ID && !manager {
				return authz.ErrDenied
			}
			ids := []int64{c.ID}
			if c.ParentID == 0 {
				var children []int64
				if err := tx.Model(&ms.ColumnComment{}).Where("parent_id=?", c.ID).Pluck("id", &children).Error; err != nil {
					return err
				}
				ids = append(ids, children...)
			}
			if err := cancelTargetReviewsTx(tx, ms.ReviewColumnComment, ids); err != nil {
				return err
			}
			return tx.Where("id IN ?", ids).Delete(&ms.ColumnComment{}).Error
		}
		if !columnPublic(row) || !columnCommentVisible(fresh, &c) {
			return authz.ErrDenied
		}
		if c.ParentID > 0 {
			var parent ms.ColumnComment
			if err := tx.Where("id=? AND is_del=0", c.ParentID).First(&parent).Error; err != nil {
				return err
			}
			if !columnCommentVisible(fresh, &parent) {
				return authz.ErrDenied
			}
		}
		if action == "feature" || action == "unfeature" {
			if c.ParentID != 0 || (!manager && (fresh.ID != row.UserID || !fresh.HasPermission(authz.ColumnPublish))) {
				return authz.ErrDenied
			}
			return tx.Model(&c).Update("featured", action == "feature").Error
		}
		if !fresh.HasPermission(authz.CommunityInteract) || (action != "up" && action != "down") {
			return authz.ErrDenied
		}
		r := ms.ColumnReaction{CommentID: id, UserID: fresh.ID}
		if err := tx.Where("comment_id=? AND user_id=?", id, fresh.ID).Find(&r).Error; err != nil {
			return err
		}
		before := r.Value
		value := 1
		if action == "down" {
			value = -1
		}
		if before == value {
			value = 0
		}
		r.Value = value
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "comment_id"}, {Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"value"})}).Create(&r).Error; err != nil {
			return err
		}
		likes, dislikes := int64(0), int64(0)
		if before == 1 {
			likes--
		}
		if before == -1 {
			dislikes--
		}
		if value == 1 {
			likes++
		}
		if value == -1 {
			dislikes++
		}
		return tx.Model(&c).Updates(map[string]any{"likes": gorm.Expr("likes + ?", likes), "dislikes": gorm.Expr("dislikes + ?", dislikes)}).Error
	})
}

func (s *columnSrv) ListColumnReviews(actor *ms.User, offset, limit int) ([]*core.ColumnReviewItem, int64, error) {
	fresh, err := freshReviewActor(s.db, actor)
	if err != nil {
		return nil, 0, err
	}
	if offset < 0 || limit < 1 || limit > 100 {
		return nil, 0, authz.ErrInvalid
	}
	db := reviewTaskScope(s.db.Model(&ms.ReviewTask{}), fresh).Where("kind IN ? AND state=?", []string{ms.ReviewColumn, ms.ReviewColumnComment}, ms.ReviewPending)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var tasks []*ms.ReviewTask
	if err := db.Order("id ASC").Offset(offset).Limit(limit).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	items := []*core.ColumnReviewItem{}
	for _, task := range tasks {
		item := &core.ColumnReviewItem{Task: task}
		var columnID int64
		if task.Kind == ms.ReviewColumn {
			item.Submission = &ms.ColumnSubmission{}
			if err := s.db.First(item.Submission, task.TargetID).Error; err != nil {
				return nil, 0, err
			}
			columnID = item.Submission.ColumnID
		} else {
			item.Comment = &ms.ColumnComment{}
			if err := s.db.First(item.Comment, task.TargetID).Error; err != nil {
				return nil, 0, err
			}
			columnID = item.Comment.ColumnID
			item.Comment.User = columnUser(s.db, item.Comment.UserID)
		}
		item.Column, err = loadColumn(s.db, columnID)
		if err != nil {
			return nil, 0, err
		}
		item.Column.User = columnUser(s.db, item.Column.UserID)
		items = append(items, item)
	}
	return items, total, nil
}
