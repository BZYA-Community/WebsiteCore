package jinzhu

import (
	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
)

// Retained references may belong to the original collection author. Newly added
// references must be the current actor's verified community uploads, never a
// known course/private resource URL being reissued through a public article.
func checkColumnMediaTx(db *gorm.DB, actor *ms.User, row *ms.Column, contents []*ms.ColumnContent) error {
	if actor == nil || actor.Model == nil || actor.ID <= 0 {
		return authz.ErrDenied
	}
	retained := map[string]bool{}
	if row != nil {
		for _, item := range row.Attachments {
			if item != nil {
				retained[item.Content] = true
			}
		}
		if row.Cover != "" {
			retained[row.Cover] = true
		}
		if row.SubmissionID > 0 {
			var previous ms.ColumnSubmission
			if err := db.First(&previous, row.SubmissionID).Error; err != nil {
				return err
			}
			for _, item := range previous.Attachments {
				if item != nil {
					retained[item.Content] = true
				}
			}
			if previous.Cover != "" {
				retained[previous.Cover] = true
			}
		}
	}
	for _, item := range contents {
		if item == nil {
			return authz.ErrInvalid
		}
		if item.Type == ms.ContentTypeText {
			continue
		}
		var expected dbr.AttachmentType
		switch item.Type {
		case ms.ContentTypeImage:
			expected = dbr.AttachmentTypeImage
		case ms.ContentTypeVideo:
			expected = dbr.AttachmentTypeVideo
		case ms.ContentTypeAttachment:
			expected = dbr.AttachmentTypeOther
		default:
			return authz.ErrInvalid
		}
		var attachment ms.Attachment
		if err := db.Where("content=? AND is_del=0 AND verified=? AND purpose=? AND type=?", item.Content, true, "community", expected).First(&attachment).Error; err != nil {
			return authz.ErrDenied
		}
		if attachment.UserID != actor.ID && !retained[item.Content] {
			return authz.ErrDenied
		}
	}
	return nil
}

func (s *columnSrv) CheckColumnMedia(actor *ms.User, id int64, contents []*ms.ColumnContent) error {
	var row *ms.Column
	if id > 0 {
		var err error
		row, err = loadColumn(s.db, id)
		if err != nil {
			return err
		}
		if !columnManager(actor, row) {
			return authz.ErrDenied
		}
	}
	return checkColumnMediaTx(s.db, actor, row, contents)
}
