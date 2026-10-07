package jinzhu

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
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

type identitySrv struct{ db *gorm.DB }

func newIdentityService(db *gorm.DB) core.IdentityService { return &identitySrv{db: db} }

func (s *identitySrv) LoadUserIdentities(users ...*ms.User) error {
	return dbr.LoadUserIdentities(s.db, users...)
}

func (s *identitySrv) ListIdentityGroups() ([]*ms.IdentityGroup, error) {
	return listIdentityGroups(s.db)
}

func listIdentityGroups(db *gorm.DB) ([]*ms.IdentityGroup, error) {
	groups := []*ms.IdentityGroup{}
	if err := db.Order("id ASC").Find(&groups).Error; err != nil {
		return nil, err
	}
	rows := []dbr.IdentityGroupPermission{}
	if err := db.Order("permission ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	byID := make(map[int64]*ms.IdentityGroup, len(groups))
	for _, group := range groups {
		group.Permissions = []string{}
		byID[group.ID] = group
	}
	for _, row := range rows {
		if group := byID[row.GroupID]; group != nil {
			group.Permissions = append(group.Permissions, row.Permission)
		}
	}
	return groups, nil
}

var identityKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func normalizeIdentityGroup(group *ms.IdentityGroup) error {
	group.Name, group.Description = strings.TrimSpace(group.Name), strings.TrimSpace(group.Description)
	if group.ID < 0 || !identityKeyPattern.MatchString(group.Key) || group.Key == "operator" ||
		group.Name == "" || utf8.RuneCountInString(group.Name) > 100 || utf8.RuneCountInString(group.Description) > 500 {
		return authz.ErrInvalid
	}
	for _, permission := range group.Permissions {
		if !authz.Valid(permission) {
			return fmt.Errorf("%w: unknown permission %q", authz.ErrInvalid, permission)
		}
	}
	group.Permissions = slices.Clone(group.Permissions)
	slices.Sort(group.Permissions)
	group.Permissions = slices.Compact(group.Permissions)
	if group.Permissions == nil {
		group.Permissions = []string{}
	}
	return nil
}

// lockIdentityPolicy serializes rare policy writes across server processes.
// ponytail: one policy-row lock; split by group only if administrative throughput requires it.
func lockIdentityPolicy(db *gorm.DB, actor *ms.User, permission string) (*ms.User, error) {
	if actor == nil || actor.Model == nil || actor.ID <= 0 {
		return nil, authz.ErrDenied
	}
	var lock dbr.IdentityGroup
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("key = ?", "guest").First(&lock).Error; err != nil {
		return nil, err
	}
	var fresh ms.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND is_del = 0", actor.ID).First(&fresh).Error; err != nil {
		return nil, err
	}
	if err := dbr.LoadUserIdentities(db, &fresh); err != nil {
		return nil, err
	}
	if !fresh.HasPermission(permission) {
		return nil, authz.ErrDenied
	}
	return &fresh, nil
}

func mayGrant(actor *ms.User, groups ...*ms.IdentityGroup) bool {
	for _, group := range groups {
		if group == nil {
			continue
		}
		for _, permission := range group.Permissions {
			if !actor.HasPermission(permission) {
				return false
			}
		}
	}
	return true
}

func identityLog(db *gorm.DB, actorID int64, action string, groupID, userID int64, before, after any) error {
	oldJSON, err := json.Marshal(before)
	if err != nil {
		return err
	}
	newJSON, err := json.Marshal(after)
	if err != nil {
		return err
	}
	return db.Create(&dbr.IdentityOperationLog{
		ActorID: actorID, Action: action, GroupID: groupID, UserID: userID,
		Before: string(oldJSON), After: string(newJSON), CreatedOn: time.Now().Unix(),
	}).Error
}

func (s *identitySrv) SaveIdentityGroup(actor *ms.User, input *ms.IdentityGroup) (*ms.IdentityGroup, error) {
	if input == nil {
		return nil, authz.ErrInvalid
	}
	group := *input
	if err := normalizeIdentityGroup(&group); err != nil {
		return nil, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		fresh, err := lockIdentityPolicy(tx, actor, authz.IdentityManage)
		if err != nil {
			return err
		}
		var old *ms.IdentityGroup
		if group.ID == 0 {
			if group.Builtin || group.Key == "guest" || group.Key == "member" || group.Key == "admin" {
				return authz.ErrInvalid
			}
			var count int64
			if err := tx.Model(&dbr.IdentityGroup{}).Where("key = ?", group.Key).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return fmt.Errorf("%w: duplicate group key", authz.ErrInvalid)
			}
		} else {
			groups, err := listIdentityGroups(tx)
			if err != nil {
				return err
			}
			for _, candidate := range groups {
				if candidate.ID == group.ID {
					old = candidate
					break
				}
			}
			if old == nil {
				return gorm.ErrRecordNotFound
			}
			if group.Key != old.Key {
				return fmt.Errorf("%w: group keys are immutable", authz.ErrInvalid)
			}
			group.Builtin = old.Builtin
		}
		if !mayGrant(fresh, old, &group) {
			return authz.ErrDenied
		}
		if group.ID == 0 {
			if err := tx.Create(&group).Error; err != nil {
				return err
			}
		} else if err := tx.Model(&dbr.IdentityGroup{}).Where("id = ?", group.ID).
			Updates(map[string]any{"name": group.Name, "description": group.Description}).Error; err != nil {
			return err
		}
		if err := tx.Where("group_id = ?", group.ID).Delete(&dbr.IdentityGroupPermission{}).Error; err != nil {
			return err
		}
		for _, permission := range group.Permissions {
			if err := tx.Create(&dbr.IdentityGroupPermission{GroupID: group.ID, Permission: permission}).Error; err != nil {
				return err
			}
		}
		return identityLog(tx, fresh.ID, "group.save", group.ID, 0, old, group)
	})
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (s *identitySrv) DeleteIdentityGroup(actor *ms.User, id int64) error {
	if id <= 0 {
		return authz.ErrInvalid
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		fresh, err := lockIdentityPolicy(tx, actor, authz.IdentityManage)
		if err != nil {
			return err
		}
		groups, err := listIdentityGroups(tx)
		if err != nil {
			return err
		}
		var group *ms.IdentityGroup
		for _, candidate := range groups {
			if candidate.ID == id {
				group = candidate
				break
			}
		}
		if group == nil {
			return gorm.ErrRecordNotFound
		}
		if group.Builtin {
			return fmt.Errorf("%w: built-in groups cannot be deleted", authz.ErrInvalid)
		}
		if !mayGrant(fresh, group) {
			return authz.ErrDenied
		}
		var count int64
		if err := tx.Model(&dbr.UserIdentityGroup{}).Where("group_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return authz.ErrGroupInUse
		}
		if err := tx.Where("group_id = ?", id).Delete(&dbr.IdentityGroupPermission{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&dbr.IdentityGroup{}, id).Error; err != nil {
			return err
		}
		return identityLog(tx, fresh.ID, "group.delete", id, 0, group, nil)
	})
}

func (s *identitySrv) SetUserIdentityGroups(actor *ms.User, userID int64, groupIDs []int64) error {
	return s.SetUsersIdentityGroups(actor, []int64{userID}, groupIDs)
}

func (s *identitySrv) SetUsersIdentityGroups(actor *ms.User, userIDs, groupIDs []int64) error {
	if len(userIDs) == 0 || len(userIDs) > 100 || len(groupIDs) > 100 {
		return authz.ErrInvalid
	}
	userIDs = slices.Clone(userIDs)
	slices.Sort(userIDs)
	userIDs = slices.Compact(userIDs)
	for _, id := range userIDs {
		if id <= 0 {
			return authz.ErrInvalid
		}
		if actor != nil && actor.Model != nil && actor.ID == id {
			return authz.ErrDenied
		}
	}
	groupIDs = slices.Clone(groupIDs)
	slices.Sort(groupIDs)
	groupIDs = slices.Compact(groupIDs)
	return s.db.Transaction(func(tx *gorm.DB) error {
		fresh, err := lockIdentityPolicy(tx, actor, authz.IdentityManage)
		if err != nil {
			return err
		}
		groups, err := listIdentityGroups(tx)
		if err != nil {
			return err
		}
		byID := make(map[int64]*ms.IdentityGroup, len(groups))
		for _, group := range groups {
			byID[group.ID] = group
		}
		next := []*ms.IdentityGroup{}
		for _, id := range groupIDs {
			group := byID[id]
			if group == nil || group.Key == "guest" || group.Key == "member" {
				return authz.ErrInvalid
			}
			next = append(next, group)
		}
		if !mayGrant(fresh, next...) {
			return authz.ErrDenied
		}
		for _, userID := range userIDs {
			var user ms.User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND is_del = 0", userID).First(&user).Error; err != nil {
				return err
			}
			if user.IsOperator {
				return authz.ErrDenied
			}
			rows := []dbr.UserIdentityGroup{}
			if err := tx.Where("user_id = ?", userID).Order("group_id ASC").Find(&rows).Error; err != nil {
				return err
			}
			old := []*ms.IdentityGroup{}
			for _, row := range rows {
				if group := byID[row.GroupID]; group != nil {
					old = append(old, group)
				}
			}
			if !mayGrant(fresh, old...) {
				return authz.ErrDenied
			}
			if err := tx.Where("user_id = ?", userID).Delete(&dbr.UserIdentityGroup{}).Error; err != nil {
				return err
			}
			for _, id := range groupIDs {
				if err := tx.Create(&dbr.UserIdentityGroup{UserID: userID, GroupID: id}).Error; err != nil {
					return err
				}
			}
			if err := identityLog(tx, fresh.ID, "user.groups", 0, userID, old, next); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *identitySrv) ListIdentityLogs(offset, limit int) ([]*ms.IdentityOperationLog, int64, error) {
	rows := []*ms.IdentityOperationLog{}
	var total int64
	if err := s.db.Model(&dbr.IdentityOperationLog{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if offset < 0 || limit <= 0 || limit > 100 {
		return nil, 0, authz.ErrInvalid
	}
	err := s.db.Order("id DESC").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, total, err
}

func (s *identitySrv) SetManagedUserStatus(actor *ms.User, userID int64, status int) error {
	if status != ms.UserStatusNormal && status != ms.UserStatusClosed {
		return authz.ErrInvalid
	}
	return s.manageUser(actor, userID, "user.status", func(tx *gorm.DB, user *ms.User) error {
		before := user.Status
		if err := tx.Model(&dbr.User{}).Where("id = ? AND is_del = 0", user.ID).
			Updates(map[string]any{"status": status, "modified_on": time.Now().Unix()}).Error; err != nil {
			return err
		}
		return identityLog(tx, actor.ID, "user.status", 0, user.ID, before, status)
	})
}

func (s *identitySrv) DeleteManagedUser(actor *ms.User, userID int64) error {
	return s.manageUser(actor, userID, "user.delete", func(tx *gorm.DB, user *ms.User) error {
		if err := user.Delete(tx); err != nil {
			return err
		}
		return identityLog(tx, actor.ID, "user.delete", 0, user.ID, false, true)
	})
}

func (s *identitySrv) manageUser(actor *ms.User, userID int64, action string, change func(*gorm.DB, *ms.User) error) error {
	if userID <= 0 {
		return authz.ErrInvalid
	}
	if actor == nil || actor.Model == nil || userID == actor.ID {
		return authz.ErrDenied
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		fresh, err := lockIdentityPolicy(tx, actor, authz.UserManage)
		if err != nil {
			return err
		}
		var user ms.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND is_del = 0", userID).First(&user).Error; err != nil {
			return err
		}
		// Operator accounts are configuration-owned and cannot be deleted in the UI.
		if user.IsOperator && (!fresh.IsOperator || action == "user.delete") {
			return authz.ErrDenied
		}
		return change(tx, &user)
	})
}
