package jinzhu

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"gorm.io/gorm"
)

func columnPublisherAllowed(tx *gorm.DB, row *ms.Column, r *ms.ColumnSubmission) bool {
	user, err := (&ms.User{Model: &ms.Model{ID: r.UserID}}).Get(tx)
	return err == nil && user.HasPermission(authz.ColumnPublish) && (!row.Curated || user.HasPermission(authz.ContentManage)) && (user.ID == row.UserID || (row.Kind == ms.ColumnCollection && user.HasPermission(authz.ContentManage)))
}

func loadColumnReviewSubject(tx *gorm.DB, kind string, id int64) (*reviewSubject, error) {
	result := &reviewSubject{}
	if kind == ms.ReviewColumn {
		r := &ms.ColumnSubmission{}
		if err := tx.First(r, id).Error; err != nil {
			return nil, err
		}
		row, err := loadColumn(tx, r.ColumnID)
		if err != nil {
			return nil, err
		}
		payload, _ := json.Marshal([]any{r.Title, r.Summary, r.Body, r.Cover, r.Attachments})
		result.ColumnSubmission, result.Column = r, row
		result.AuthorID, result.ParentID, result.Status = r.UserID, row.ID, int(r.AuditStatus)
		result.Snapshot = fmt.Sprintf("%d:%x", r.ID, sha256.Sum256(payload))
		result.Pending = r.AuditStatus == ms.PostAuditPending && row.SubmissionID == r.ID && !row.Blocked && columnPublisherAllowed(tx, row, r)
	} else {
		c := &ms.ColumnComment{}
		if err := tx.First(c, id).Error; err != nil {
			return nil, err
		}
		row, err := loadColumn(tx, c.ColumnID)
		if err != nil {
			return nil, err
		}
		result.ColumnComment, result.Column = c, row
		result.AuthorID, result.ParentID, result.Status = c.UserID, row.ID, int(c.AuditStatus)
		result.Pending = c.AuditStatus == ms.PostAuditPending && columnPublic(row)
		if c.ParentID > 0 {
			var parent ms.ColumnComment
			if err := tx.First(&parent, c.ParentID).Error; err != nil {
				return nil, err
			}
			result.Pending = result.Pending && parent.AuditStatus != ms.PostAuditRejected && parent.AuditStatus != ms.ColumnWithdrawn
		}
	}
	return result, nil
}

func applyColumnReview(tx *gorm.DB, subject *reviewSubject, status int, reason string) error {
	if r := subject.ColumnSubmission; r != nil {
		if err := tx.Model(r).Updates(map[string]any{"audit_status": status, "reason": reason}).Error; err != nil {
			return err
		}
		if status == int(ms.PostAuditApproved) {
			if err := publishColumnTx(tx, subject.Column, r); err != nil {
				return err
			}
		}
		return columnMessage(tx, 0, r.UserID, subject.Column.ID, 0, "专栏或合集审核结果："+reviewResultText(status, reason))
	}
	c := subject.ColumnComment
	if err := tx.Model(c).Updates(map[string]any{"audit_status": status, "reason": reason}).Error; err != nil {
		return err
	}
	if status == int(ms.PostAuditApproved) {
		if err := notifyColumnCommentTx(tx, subject.Column, c); err != nil {
			return err
		}
	}
	return columnMessage(tx, 0, c.UserID, c.ColumnID, c.ID, "专栏评论审核结果："+reviewResultText(status, reason))
}

func reviewResultText(status int, reason string) string {
	if status == int(ms.PostAuditApproved) {
		return "已通过"
	}
	return "未通过：" + reason
}

// Policy changes and task reconciliation both use the same policy lock. Keep the
// submitted content recoverable while invalidating its pending publication.
func cancelRevokedColumnSubmissionsTx(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&ms.Column{}) {
		return nil
	}
	var tasks []*ms.ReviewTask
	if err := tx.Where("kind=? AND state=?", ms.ReviewColumn, ms.ReviewPending).Find(&tasks).Error; err != nil {
		return err
	}
	for _, task := range tasks {
		subject, err := loadColumnReviewSubject(tx, task.Kind, task.TargetID)
		if err != nil {
			return err
		}
		if !subject.Pending {
			if err := withdrawColumnTx(tx, subject.Column); err != nil {
				return err
			}
		}
	}
	return nil
}
