package jinzhu

import (
	"strings"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"gorm.io/gorm"
)

// Deleted rows are readable only through this owner-scoped return path. The
// latest submitted content is immutable after deletion; it is not a server draft.
func returnedColumn(db *gorm.DB, actor *ms.User, id int64) (*ms.Column, error) {
	if actor == nil || actor.Model == nil || actor.ID <= 0 {
		return nil, authz.ErrDenied
	}
	row := &ms.Column{}
	if err := db.Unscoped().Where("id=? AND user_id=? AND kind=? AND is_del=1 AND admin_deleted_by>0", id, actor.ID, ms.ColumnArticle).First(row).Error; err != nil {
		return nil, err
	}
	submission := &ms.ColumnSubmission{}
	if err := db.Where("id=? AND column_id=?", row.SubmissionID, row.ID).First(submission).Error; err != nil {
		return nil, err
	}
	row.Title, row.Summary, row.Body = submission.Title, submission.Summary, submission.Body
	row.Attachments, row.Submission = submission.Attachments, submission
	row.Cover = ""
	return row, nil
}

func (s *columnSrv) GetReturnedColumn(actor *ms.User, id int64) (*ms.Column, error) {
	return returnedColumn(s.db, actor, id)
}

func returnDeletedColumnTx(tx *gorm.DB, actor *ms.User, row *ms.Column, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return authz.ErrInvalid
	}
	var latest ms.ColumnSubmission
	if err := tx.Where("id=? AND column_id=?", row.SubmissionID, row.ID).First(&latest).Error; err != nil {
		return err
	}
	if err := tx.Model(row).Updates(map[string]any{
		"admin_deleted_by": actor.ID, "admin_deleted_on": time.Now().Unix(), "admin_delete_reason": reason,
	}).Error; err != nil {
		return err
	}
	return storeColumnMessage(tx, &ms.Message{Model: &ms.Model{}, ReceiverUserID: row.UserID, Type: ms.MsgTypeSystem,
		ColumnID: row.ID, ColumnReturned: true, Brief: "你的专栏已被管理员删除，可在我的内容中取回最新提交：" + latest.Title,
		Content: "删除原因：" + reason})
}
