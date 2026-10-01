package jinzhu

import (
	"sort"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Access changes take an exclusive transaction lock; chat operations take a
// shared lock before locking user rows. This lets unrelated conversations send
// concurrently while making cancellation observe both identities consistently,
// including simultaneous demotions performed by different administrators.
func lockIdentityChanges(tx *gorm.DB, exclusive bool) error {
	query := "SELECT pg_advisory_xact_lock_shared(941, 1)"
	if exclusive {
		query = "SELECT pg_advisory_xact_lock(941, 1)"
	}
	return tx.Exec(query).Error
}

// Every account/course/chat mutation locks users first, in increasing ID order.
// Authorization is evaluated against these fresh rows, never cached identities.
func lockUsers(tx *gorm.DB, ids ...int64) (map[int64]*ms.User, error) {
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	users := make(map[int64]*ms.User, len(ids))
	for _, id := range ids {
		if users[id] != nil {
			continue
		}
		var user ms.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&user).Error; err != nil {
			return nil, err
		}
		users[id] = &user
	}
	return users, nil
}

func cancelPending(tx *gorm.DB, userID int64) error {
	// Cancelling all outstanding requests when an identity changes is safe only
	// for invalid pairs; retain valid requests and all established conversations.
	var conversations []dbr.WhisperConversation
	if err := tx.Where("(low_user_id = ? OR high_user_id = ?) AND pending_sender_id IS NOT NULL", userID, userID).Find(&conversations).Error; err != nil {
		return err
	}
	for _, c := range conversations {
		var a, b ms.User
		if err := tx.Unscoped().First(&a, c.LowUserID).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().First(&b, c.HighUserID).Error; err != nil {
			return err
		}
		sender, receiver := &a, &b
		if *c.PendingSenderID == b.ID {
			sender, receiver = &b, &a
		}
		if !c.CanRequest(sender, receiver) {
			if err := tx.Model(&c).Update("pending_sender_id", nil).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func teacherReleased(tx *gorm.DB, user *ms.User) error {
	if user.IsMentor {
		return dbr.ErrTeacherInUse
	}
	var n int64
	if err := tx.Model(&dbr.Course{}).Where("teacher_id = ?", user.ID).Count(&n).Error; err != nil {
		return err
	}
	if n != 0 {
		return dbr.ErrTeacherInUse
	}
	return nil
}

func (s *userManageSrv) ChangeMemberAccess(actorID, userID int64, identity string, mentor, auditor bool) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := lockIdentityChanges(tx, true); err != nil {
			return err
		}
		users, err := lockUsers(tx, actorID, userID)
		if err != nil {
			return err
		}
		actor, user := users[actorID], users[userID]
		if !actor.CanManageUsers() || user.AccountType != "member" {
			return dbr.ErrPermission
		}
		if identity != ms.MemberStudent && identity != ms.MemberTeacher {
			return dbr.ErrPermission
		}
		if user.IsTeacher() && identity == ms.MemberStudent {
			if err := teacherReleased(tx, user); err != nil {
				return err
			}
		}
		oldRoles := user.Roles
		user.MemberIdentity, user.IsMentor, user.Roles = ms.Identity(identity), mentor, ""
		if auditor {
			user.Roles = ms.RoleAuditor
		}
		if !user.ValidIdentity() {
			return dbr.ErrPermission
		}
		if err := tx.Model(user).Updates(map[string]any{"member_identity": identity, "is_mentor": mentor, "roles": user.Roles}).Error; err != nil {
			return err
		}
		if oldRoles != user.Roles {
			action := "remove"
			if auditor {
				action = "add"
			}
			if _, err := (&dbr.UserRoleLog{UserID: userID, OperatorID: actorID, OldRoles: oldRoles, NewRoles: user.Roles, Action: action}).Create(tx); err != nil {
				return err
			}
		}
		return cancelPending(tx, userID)
	})
}

func (s *userManageSrv) ChangeAccountStatus(actorID, userID int64, status int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := lockIdentityChanges(tx, true); err != nil {
			return err
		}
		users, err := lockUsers(tx, actorID, userID)
		if err != nil {
			return err
		}
		actor, user := users[actorID], users[userID]
		if !actor.CanManageUsers() || user.AccountType == "operator" || actorID == userID {
			return dbr.ErrPermission
		}
		if user.AccountType == "admin" && !actor.CanManageAdmins() {
			return dbr.ErrPermission
		}
		if status != ms.UserStatusNormal && status != ms.UserStatusClosed {
			return dbr.ErrPermission
		}
		updates := map[string]any{"status": status}
		if user.AccountType == "admin" && status == ms.UserStatusNormal {
			updates["roles"] = ms.RoleAdmin
		}
		if err := tx.Model(user).Updates(updates).Error; err != nil {
			return err
		}
		return cancelPending(tx, userID)
	})
}

func (s *userManageSrv) RemoveAdminRole(actorID, userID int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := lockIdentityChanges(tx, true); err != nil {
			return err
		}
		users, err := lockUsers(tx, actorID, userID)
		if err != nil {
			return err
		}
		if !users[actorID].CanManageAdmins() || users[userID].AccountType != "admin" {
			return dbr.ErrPermission
		}
		oldRoles := users[userID].Roles
		if err := tx.Model(users[userID]).Updates(map[string]any{"roles": "", "status": ms.UserStatusClosed}).Error; err != nil {
			return err
		}
		if oldRoles != "" {
			if _, err := (&dbr.UserRoleLog{UserID: userID, OperatorID: actorID, OldRoles: oldRoles, NewRoles: "", Action: "remove"}).Create(tx); err != nil {
				return err
			}
		}
		return cancelPending(tx, userID)
	})
}

func (s *userManageSrv) DeleteMember(actorID, userID int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := lockIdentityChanges(tx, true); err != nil {
			return err
		}
		users, err := lockUsers(tx, actorID, userID)
		if err != nil {
			return err
		}
		actor, user := users[actorID], users[userID]
		if !actor.CanManageUsers() || user.AccountType != "member" || actorID == userID {
			return dbr.ErrPermission
		}
		if err := teacherReleased(tx, user); err != nil {
			return err
		}
		if err := tx.Model(user).Updates(map[string]any{"is_del": 1, "deleted_on": time.Now().Unix()}).Error; err != nil {
			return err
		}
		return cancelPending(tx, userID)
	})
}

func (s *userManageSrv) CreateAdmin(actorID int64, user *ms.User) (*ms.User, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		users, err := lockUsers(tx, actorID)
		if err != nil {
			return err
		}
		if !users[actorID].CanManageAdmins() {
			return dbr.ErrPermission
		}
		user.AccountType, user.Roles, user.MemberIdentity, user.IsMentor = "admin", ms.RoleAdmin, nil, false
		user.Status, user.MustChangePassword = ms.UserStatusNormal, true
		return tx.Create(user).Error
	})
	if err == nil && s.ums != nil {
		s.ums.AddUserMetric(user.ID)
	}
	return user, err
}
