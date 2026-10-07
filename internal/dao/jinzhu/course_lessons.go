package jinzhu

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func courseAttachmentValid(upload *ms.Attachment, kind string) bool {
	limit := int64(50 * 1024 * 1024)
	if kind == "resource" {
		limit = 2 * 1024 * 1024 * 1024
	} else if kind != "attachment" {
		return false
	}
	return upload != nil && upload.Verified && upload.Purpose == "course_"+kind &&
		upload.FileSize > 0 && upload.FileSize <= limit && upload.Content != "" && upload.MimeType != ""
}

func courseLessonAttachments(tx *gorm.DB, lessons []*ms.CourseLesson) error {
	ids := make([]int64, 0, len(lessons))
	byLesson := make(map[int64]*ms.CourseLesson, len(lessons))
	for _, lesson := range lessons {
		lesson.Attachments = []*ms.CourseLessonAttachment{}
		ids = append(ids, lesson.ID)
		byLesson[lesson.ID] = lesson
	}
	if len(ids) == 0 {
		return nil
	}
	var refs []*ms.CourseLessonAttachment
	if err := tx.Where("lesson_id IN ?", ids).Order("sort ASC, id ASC").Find(&refs).Error; err != nil {
		return err
	}
	if len(refs) == 0 {
		return nil
	}
	uploadIDs := make([]int64, 0, len(refs))
	for _, ref := range refs {
		uploadIDs = append(uploadIDs, ref.AttachmentID)
	}
	var uploads []*ms.Attachment
	if err := tx.Where("id IN ?", uploadIDs).Find(&uploads).Error; err != nil {
		return err
	}
	byUpload := make(map[int64]*ms.Attachment, len(uploads))
	for _, upload := range uploads {
		byUpload[upload.ID] = upload
	}
	for _, ref := range refs {
		upload := byUpload[ref.AttachmentID]
		if upload == nil {
			return core.ErrCourseAttachmentInvalid
		}
		ref.FileSize, ref.MimeType = upload.FileSize, upload.MimeType
		byLesson[ref.LessonID].Attachments = append(byLesson[ref.LessonID].Attachments, ref)
	}
	return nil
}

func (s *courseSrv) GetCourseLessons(courseID int64) ([]*ms.CourseLesson, error) {
	if _, err := s.GetCourseByID(courseID); err != nil {
		return nil, err
	}
	lessons := []*ms.CourseLesson{}
	if err := s.db.Where("course_id = ?", courseID).Order("sort ASC, id ASC").Find(&lessons).Error; err != nil {
		return nil, err
	}
	if err := courseLessonAttachments(s.db, lessons); err != nil {
		return nil, err
	}
	return lessons, nil
}

func (s *courseSrv) GetCourseLessonAttachment(id int64) (*ms.Attachment, error) {
	var ref ms.CourseLessonAttachment
	if err := s.db.Where("id = ?", id).First(&ref).Error; err != nil {
		return nil, err
	}
	var lesson ms.CourseLesson
	if err := s.db.Where("id = ?", ref.LessonID).First(&lesson).Error; err != nil {
		return nil, err
	}
	if _, err := s.GetCourseByID(lesson.CourseID); err != nil {
		return nil, err
	}
	var upload ms.Attachment
	if err := s.db.Where("id = ?", ref.AttachmentID).First(&upload).Error; err != nil {
		return nil, err
	}
	if !courseAttachmentValid(&upload, ref.Kind) {
		return nil, core.ErrCourseAttachmentInvalid
	}
	return &upload, nil
}

func (s *courseManageSrv) SaveCourseLesson(actor *ms.User, lesson *ms.CourseLesson) (*ms.CourseLesson, error) {
	lesson.Title = strings.TrimSpace(lesson.Title)
	if lesson.CourseID <= 0 || lesson.Title == "" || utf8.RuneCountInString(lesson.Title) > 128 ||
		utf8.RuneCountInString(lesson.Intro) > 2000 || len(lesson.Attachments) > 100 {
		return nil, authz.ErrInvalid
	}
	if lesson.Model == nil {
		lesson.Model = &ms.Model{}
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if _, err := lockManagedCourse(tx, actor, lesson.CourseID); err != nil {
			return err
		}
		var before *ms.CourseLesson
		old := map[int64]*ms.CourseLessonAttachment{}
		if lesson.ID > 0 {
			before = &ms.CourseLesson{}
			if err := tx.Where("id = ? AND course_id = ?", lesson.ID, lesson.CourseID).First(before).Error; err != nil {
				return err
			}
			if err := courseLessonAttachments(tx, []*ms.CourseLesson{before}); err != nil {
				return err
			}
			for _, ref := range before.Attachments {
				old[ref.AttachmentID] = ref
			}
			// Omitted/null attachments preserve references. Only an explicit []
			// clears them, so metadata updates cannot erase unread resources.
			if lesson.Attachments == nil {
				lesson.Attachments = make([]*ms.CourseLessonAttachment, 0, len(before.Attachments))
				for _, ref := range before.Attachments {
					copy := *ref
					lesson.Attachments = append(lesson.Attachments, &copy)
				}
			}
			model := *before.Model
			lesson.Model = &model
		}
		seen := map[int64]bool{}
		for i, ref := range lesson.Attachments {
			if ref == nil || ref.AttachmentID <= 0 || seen[ref.AttachmentID] {
				return core.ErrCourseAttachmentInvalid
			}
			ref.Name = strings.TrimSpace(ref.Name)
			if ref.Name == "" || utf8.RuneCountInString(ref.Name) > 255 {
				return core.ErrCourseAttachmentInvalid
			}
			seen[ref.AttachmentID] = true
			var upload ms.Attachment
			if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ?", ref.AttachmentID).First(&upload).Error; err != nil {
				return core.ErrCourseAttachmentInvalid
			}
			if !courseAttachmentValid(&upload, ref.Kind) || (upload.UserID != actor.ID && old[ref.AttachmentID] == nil) {
				return core.ErrCourseAttachmentInvalid
			}
			ref.ID, ref.Sort = 0, i
			if previous := old[ref.AttachmentID]; previous != nil {
				ref.ID = previous.ID
			}
			ref.FileSize, ref.MimeType = upload.FileSize, upload.MimeType
		}
		action := "create"
		if before == nil {
			if err := tx.Create(lesson).Error; err != nil {
				return err
			}
		} else {
			action = "update"
			lesson.ModifiedOn = time.Now().Unix()
			if err := tx.Model(&dbr.CourseLesson{}).Where("id = ?", lesson.ID).
				Updates(map[string]any{"title": lesson.Title, "intro": lesson.Intro, "sort": lesson.Sort, "modified_on": lesson.ModifiedOn}).Error; err != nil {
				return err
			}
		}
		keep := make([]int64, 0, len(lesson.Attachments))
		for _, ref := range lesson.Attachments {
			ref.LessonID = lesson.ID
			if err := tx.Save(ref).Error; err != nil {
				return err
			}
			keep = append(keep, ref.ID)
		}
		remove := tx.Where("lesson_id = ?", lesson.ID)
		if len(keep) > 0 {
			remove = remove.Where("id NOT IN ?", keep)
		}
		if err := remove.Delete(&dbr.CourseLessonAttachment{}).Error; err != nil {
			return err
		}
		if lesson.Attachments == nil {
			lesson.Attachments = []*ms.CourseLessonAttachment{}
		}
		return dbr.AppendOperationLog(tx, actor.ID, action, "course_lesson", lesson.ID, before, lesson)
	})
	if err != nil {
		return nil, err
	}
	return lesson, nil
}

func (s *courseManageSrv) DeleteCourseLesson(actor *ms.User, id int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var lesson ms.CourseLesson
		if err := tx.Where("id = ?", id).First(&lesson).Error; err != nil {
			return err
		}
		if _, err := lockManagedCourse(tx, actor, lesson.CourseID); err != nil {
			return err
		}
		if err := tx.Where("id = ?", id).First(&lesson).Error; err != nil {
			return err
		}
		if err := courseLessonAttachments(tx, []*ms.CourseLesson{&lesson}); err != nil {
			return err
		}
		if err := tx.Where("lesson_id = ?", id).Delete(&dbr.CourseLessonAttachment{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("id = ?", id).Delete(&dbr.CourseLesson{}).Error; err != nil {
			return err
		}
		return dbr.AppendOperationLog(tx, actor.ID, "delete", "course_lesson", id, &lesson, nil)
	})
}
