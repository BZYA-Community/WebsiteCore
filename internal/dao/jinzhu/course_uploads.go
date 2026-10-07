package jinzhu

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"github.com/BZYA-Community/WebsiteCore/internal/media"
	"github.com/gofrs/uuid/v5"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type courseUploadSrv struct{ db *gorm.DB }

func uploadActor(db *gorm.DB, id int64) (*ms.User, error) {
	u, err := (&ms.User{Model: &ms.Model{ID: id}}).Get(db)
	if err != nil || !u.HasPermission(authz.CourseUpload) || (!u.HasPermission(authz.CourseManage) && !u.HasPermission(authz.CourseManageOwn)) {
		return nil, authz.ErrDenied
	}
	return u, nil
}

func (s *courseUploadSrv) StartCourseUpload(actor *ms.User, name, kind, mimeType string, size int64) (*ms.Attachment, error) {
	if actor == nil || actor.Model == nil || !conf.CoursesEnabled() {
		return nil, authz.ErrDenied
	}
	limit := conf.UploadLimits().CourseAttachmentMaxBytes
	if kind == "resource" {
		limit = conf.UploadLimits().CourseResourceMaxBytes
	} else if kind != "attachment" {
		return nil, authz.ErrInvalid
	}
	mimeType, ext, err := media.TypeAndExtension(name, mimeType)
	if err != nil || size <= 0 || size > limit {
		return nil, authz.ErrInvalid
	}
	key := "attachment/course/staging/" + uuid.Must(uuid.NewV4()).String() + ext
	attachment := &ms.Attachment{Model: &ms.Model{}, UserID: actor.ID, Name: name, Content: key, FileSize: size, MimeType: mimeType, Purpose: "course_" + kind, Type: ms.AttachmentTypeOther, UploadExpiresOn: time.Now().Unix() + conf.UploadLimits().CredentialTTLSeconds}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var user ms.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, actor.ID).Error; err != nil {
			return err
		}
		if _, err := uploadActor(tx, actor.ID); err != nil {
			return err
		}
		var pending int64
		if err := tx.Model(&ms.Attachment{}).Where("user_id = ? AND verified = FALSE AND upload_expires_on > ? AND is_del = 0", actor.ID, time.Now().Unix()).Count(&pending).Error; err != nil {
			return err
		}
		// ponytail: eight outstanding uploads per trusted user; tune only with measured upload demand.
		if pending >= 8 {
			return fmt.Errorf("too many pending course uploads")
		}
		if err := tx.Create(attachment).Error; err != nil {
			return err
		}
		return dbr.AppendOperationLog(tx, actor.ID, "upload.start", "attachment", attachment.ID, nil, map[string]any{"kind": kind, "size": size})
	})
	return attachment, err
}

func (s *courseUploadSrv) WriteCourseUpload(id int64, contentType string, reader io.Reader, oss core.ObjectStorageService) error {
	if !conf.CoursesEnabled() {
		return authz.ErrDenied
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var a ms.Attachment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&a, id).Error; err != nil {
			return err
		}
		if a.Verified || a.UploadExpiresOn <= time.Now().Unix() || !strings.HasPrefix(a.Content, "attachment/course/staging/") {
			return authz.ErrDenied
		}
		if _, err := uploadActor(tx, a.UserID); err != nil {
			return err
		}
		declared, _, err := media.TypeAndExtension(a.Name, contentType)
		if err != nil || declared != a.MimeType {
			return authz.ErrInvalid
		}
		header := make([]byte, min(a.FileSize, 512))
		if _, err := io.ReadFull(reader, header); err != nil {
			return err
		}
		if err := media.ValidateHeader(header, a.MimeType); err != nil {
			return err
		}
		_, err = oss.PutObject(a.Content, io.MultiReader(bytes.NewReader(header), reader), a.FileSize, a.MimeType, true)
		return err
	})
}

func finalCourseKey(stage string) string {
	return strings.Replace(stage, "attachment/course/staging/", "attachment/course/resources/", 1)
}

func (s *courseUploadSrv) CompleteCourseUpload(actor *ms.User, id int64, oss core.ObjectStorageService) (*ms.Attachment, error) {
	if actor == nil || actor.Model == nil || !conf.CoursesEnabled() {
		return nil, authz.ErrDenied
	}
	var a ms.Attachment
	stage := ""
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Recheck policy after the potentially long direct upload has finished.
		var policy dbr.IdentityGroup
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("key = ?", "guest").First(&policy).Error; err != nil {
			return err
		}
		var user ms.User
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&user, actor.ID).Error; err != nil {
			return err
		}
		if _, err := uploadActor(tx, actor.ID); err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&a, id).Error; err != nil {
			return err
		}
		if a.UserID != actor.ID {
			return authz.ErrDenied
		}
		if a.Verified {
			return nil
		}
		if a.UploadExpiresOn <= time.Now().Unix() || !strings.HasPrefix(a.Content, "attachment/course/staging/") {
			return authz.ErrDenied
		}
		stage = a.Content
		info, err := oss.InspectObject(stage)
		if err != nil {
			return err
		}
		if info.Size != a.FileSize {
			return authz.ErrInvalid
		}
		if err := media.ValidateHeader(info.Header, a.MimeType); err != nil {
			return err
		}
		// The intent's validated MIME type is bound to the upload credential.
		// Check the bytes against it: sniffers legitimately report generic types
		// for formats such as OOXML, CSV, WAV and QuickTime.
		final := finalCourseKey(stage)
		if err := oss.PromoteObject(stage, final, info.ETag); err != nil {
			return err
		}
		a.Content, a.Verified = oss.ObjectURL(final), true
		if err := tx.Model(&a).Updates(map[string]any{"content": a.Content, "verified": true}).Error; err != nil {
			return err
		}
		return dbr.AppendOperationLog(tx, actor.ID, "upload.complete", "attachment", a.ID, nil, map[string]any{"size": a.FileSize, "mime_type": a.MimeType})
	})
	if err == nil && stage != "" {
		_ = oss.DeleteObject(stage)
	} // Expiry cleanup retries leftover staging objects.
	return &a, err
}

func (s *courseUploadSrv) ExpireCourseUploads(oss core.ObjectStorageService) error {
	var rows []ms.Attachment
	if err := s.db.Where("purpose IN ? AND upload_expires_on > 0 AND upload_expires_on < ? AND is_del = 0", []string{"course_attachment", "course_resource"}, time.Now().Unix()-86400).Order("id ASC").Limit(100).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var a ms.Attachment
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("upload_expires_on > 0 AND upload_expires_on < ?", time.Now().Unix()-86400).First(&a, row.ID).Error; err != nil {
				return err
			}
			key := oss.ObjectKey(a.Content)
			stage := strings.Replace(key, "attachment/course/resources/", "attachment/course/staging/", 1)
			if !strings.HasPrefix(stage, "attachment/course/staging/") {
				return authz.ErrInvalid
			}
			for _, candidate := range []string{stage, finalCourseKey(stage)} {
				if a.Verified && candidate != stage {
					continue
				}
				if exists, err := oss.IsObjectExist(candidate); exists {
					if err := oss.DeleteObject(candidate); err != nil {
						return err
					}
				} else if err != nil {
					return err
				}
			}
			if a.Verified {
				return tx.Model(&a).Update("upload_expires_on", 0).Error
			}
			if err := tx.Delete(&a).Error; err != nil {
				return err
			}
			return dbr.AppendOperationLog(tx, 0, "upload.expire", "attachment", a.ID, nil, nil)
		})
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
	}
	return nil
}
