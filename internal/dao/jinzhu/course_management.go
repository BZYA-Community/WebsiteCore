package jinzhu

import (
	"strings"
	"unicode/utf8"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Keep current grants/status stable for the mutation, following identity writes'
// policy -> user -> content lock order. No cached account grants authorize writes.
func lockCourseActor(tx *gorm.DB, actor *ms.User) (*ms.User, error) {
	if actor == nil || actor.Model == nil || actor.ID <= 0 {
		return nil, authz.ErrDenied
	}
	var policy dbr.IdentityGroup
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("key = ?", "guest").First(&policy).Error; err != nil {
		return nil, err
	}
	var fresh ms.User
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ?", actor.ID).First(&fresh).Error; err != nil {
		return nil, err
	}
	if err := dbr.LoadUserIdentities(tx, &fresh); err != nil {
		return nil, err
	}
	if !fresh.HasPermission(authz.CourseManage) && !fresh.HasPermission(authz.CourseManageOwn) {
		return nil, authz.ErrDenied
	}
	return &fresh, nil
}

func lockManagedCourse(tx *gorm.DB, actor *ms.User, courseID int64) (*ms.Course, error) {
	fresh, err := lockCourseActor(tx, actor)
	if err != nil {
		return nil, err
	}
	var course ms.Course
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", courseID).First(&course).Error; err != nil {
		return nil, err
	}
	if !fresh.HasPermission(authz.CourseManage) && course.TeacherID != fresh.ID {
		return nil, authz.ErrDenied
	}
	return &course, nil
}

func lockCourseTaxonomy(tx *gorm.DB, actor *ms.User) error {
	fresh, err := lockCourseActor(tx, actor)
	if err != nil {
		return err
	}
	if !fresh.HasPermission(authz.CourseManage) {
		return authz.ErrDenied
	}
	// ponytail: one lock for rare taxonomy edits prevents concurrent reparenting
	// cycles; split only if taxonomy maintenance becomes a throughput bottleneck.
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended('course-taxonomy', 0))").Error
}

func validateCourseCategory(tx *gorm.DB, group *ms.CourseGroup) error {
	group.Name = strings.TrimSpace(group.Name)
	if group.Name == "" || utf8.RuneCountInString(group.Name) > 64 || group.ParentID < 0 {
		return core.ErrCourseCategoryInvalid
	}
	var groups []*ms.CourseGroup
	if err := tx.Find(&groups).Error; err != nil {
		return err
	}
	parents := make(map[int64]int64, len(groups))
	for _, item := range groups {
		parents[item.ID] = item.ParentID
	}
	seen := map[int64]bool{}
	if group.Model != nil && group.ID > 0 {
		seen[group.ID] = true
	}
	for id := group.ParentID; id != 0; {
		parent, exists := parents[id]
		if !exists || seen[id] {
			return core.ErrCourseCategoryInvalid
		}
		seen[id] = true
		id = parent
	}
	return nil
}

func (s *courseManageSrv) CreateCourseGroup(actor *ms.User, group *ms.CourseGroup) (*ms.CourseGroup, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := lockCourseTaxonomy(tx, actor); err != nil {
			return err
		}
		if err := validateCourseCategory(tx, group); err != nil {
			return err
		}
		if _, err := group.Create(tx); err != nil {
			return err
		}
		return dbr.AppendOperationLog(tx, actor.ID, "create", "course_category", group.ID, nil, group.Format(0))
	})
	return group, err
}

func (s *courseManageSrv) UpdateCourseGroup(actor *ms.User, group *ms.CourseGroup) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := lockCourseTaxonomy(tx, actor); err != nil {
			return err
		}
		var before ms.CourseGroup
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", group.ID).First(&before).Error; err != nil {
			return err
		}
		if err := validateCourseCategory(tx, group); err != nil {
			return err
		}
		if err := group.Update(tx); err != nil {
			return err
		}
		return dbr.AppendOperationLog(tx, actor.ID, "update", "course_category", group.ID, before.Format(0), group.Format(0))
	})
}

func (s *courseManageSrv) DeleteCourseGroup(actor *ms.User, id int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := lockCourseTaxonomy(tx, actor); err != nil {
			return err
		}
		var group ms.CourseGroup
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&group).Error; err != nil {
			return err
		}
		var children, courses int64
		if err := tx.Model(&dbr.CourseGroup{}).Where("parent_id = ?", id).Count(&children).Error; err != nil {
			return err
		}
		if err := tx.Model(&dbr.Course{}).Where("group_id = ?", id).Count(&courses).Error; err != nil {
			return err
		}
		if children > 0 || courses > 0 {
			return core.ErrCourseCategoryNotEmpty
		}
		if err := group.Delete(tx); err != nil {
			return err
		}
		return dbr.AppendOperationLog(tx, actor.ID, "delete", "course_category", id, group.Format(0), nil)
	})
}

func validateCourse(tx *gorm.DB, course *ms.Course) error {
	course.Title = strings.TrimSpace(course.Title)
	if course.Title == "" || utf8.RuneCountInString(course.Title) > 128 ||
		utf8.RuneCountInString(course.Intro) > 2000 || utf8.RuneCountInString(course.TeacherIntro) > 2000 ||
		len(course.Cover) > 255 {
		return authz.ErrInvalid
	}
	var group ms.CourseGroup
	// The shared lock prevents a category deletion from racing this reference.
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ?", course.GroupID).First(&group).Error; err != nil {
		return err
	}
	var teacher ms.User
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ? AND status = ?", course.TeacherID, ms.UserStatusNormal).First(&teacher).Error; err != nil {
		return err
	}
	return nil
}

func (s *courseManageSrv) CreateCourse(actor *ms.User, course *ms.Course) (*ms.Course, error) {
	if course.VideoURL != "" {
		return nil, authz.ErrInvalid
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		fresh, err := lockCourseActor(tx, actor)
		if err != nil {
			return err
		}
		if !fresh.HasPermission(authz.CourseManage) || course.TeacherID == 0 {
			course.TeacherID = fresh.ID
		}
		if err := validateCourse(tx, course); err != nil {
			return err
		}
		if _, err := course.Create(tx); err != nil {
			return err
		}
		return dbr.AppendOperationLog(tx, actor.ID, "create", "course", course.ID, nil, course.Format())
	})
	return course, err
}

func (s *courseManageSrv) UpdateCourse(actor *ms.User, course *ms.Course) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		fresh, err := lockCourseActor(tx, actor)
		if err != nil {
			return err
		}
		var before ms.Course
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", course.ID).First(&before).Error; err != nil {
			return err
		}
		if !fresh.HasPermission(authz.CourseManage) && (before.TeacherID != fresh.ID || course.TeacherID != fresh.ID) {
			return authz.ErrDenied
		}
		if err := validateCourse(tx, course); err != nil {
			return err
		}
		// Legacy resources remain readable, but only verified lesson attachments
		// can be added through the new editor.
		course.VideoURL = before.VideoURL
		if err := course.Update(tx); err != nil {
			return err
		}
		return dbr.AppendOperationLog(tx, actor.ID, "update", "course", course.ID, before.Format(), course.Format())
	})
}
