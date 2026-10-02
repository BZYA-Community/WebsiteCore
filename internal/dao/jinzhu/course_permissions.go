package jinzhu

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *courseManageSrv) CreateCourseAs(actorID int64, course *ms.Course) (*ms.Course, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		users, err := lockUsers(tx, actorID)
		if err != nil {
			return err
		}
		if !users[actorID].CanCreateCourse() {
			return dbr.ErrPermission
		}
		course.TeacherID = actorID
		return tx.Create(course).Error
	})
	return course, err
}

func (s *courseManageSrv) UpdateCourseAs(actorID int64, course *ms.Course) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Only the new teacher needs a user lock: removing a course never makes
		// a concurrent demotion unsafe. Lock the course after all user locks.
		users, err := lockUsers(tx, actorID, course.TeacherID)
		if err != nil {
			return err
		}
		var current ms.Course
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, course.ID).Error; err != nil {
			return err
		}
		actor, teacher := users[actorID], users[course.TeacherID]
		if !actor.CanEditCourse(current.TeacherID) {
			return dbr.ErrPermission
		}
		if course.TeacherID != current.TeacherID {
			if !actor.CanManageUsers() || !teacher.CanCreateCourse() {
				return dbr.ErrPermission
			}
		}
		// An Admin may edit a banned Teacher's course without reassigning it.
		return tx.Model(&current).Updates(map[string]any{
			"group_id": course.GroupID, "teacher_id": course.TeacherID,
			"title": course.Title, "intro": course.Intro,
			"video_url": course.VideoURL, "cover": course.Cover,
		}).Error
	})
}

func (s *courseManageSrv) DeleteCourseAs(actorID int64, course *ms.Course) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		users, err := lockUsers(tx, actorID)
		if err != nil {
			return err
		}
		if !users[actorID].CanDeleteCourse() {
			return dbr.ErrPermission
		}
		var current ms.Course
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, course.ID).Error; err != nil {
			return err
		}
		return (&courseManageSrv{db: tx}).DeleteCourse(&current)
	})
}

func (s *userManageSrv) ListCourseTeachers(keyword string) ([]*ms.User, error) {
	var users []*ms.User
	db := s.db.Where("(member_identity = ? OR roles IN ?) AND status = ?", ms.MemberTeacher, []string{ms.RoleAdmin, ms.RoleOperator}, ms.UserStatusNormal)
	if keyword != "" {
		db = db.Where("username ILIKE ? OR nickname ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	err := db.Order("id ASC").Limit(50).Find(&users).Error
	return users, err
}
