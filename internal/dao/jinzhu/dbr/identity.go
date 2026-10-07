package dbr

import (
	"slices"
	"sort"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"gorm.io/gorm"
)

// IdentityGroup is a policy definition. Guest/member membership is automatic.
type IdentityGroup struct {
	ID          int64    `gorm:"primaryKey" json:"id"`
	Key         string   `gorm:"uniqueIndex;size:64;not null" json:"key"`
	Name        string   `gorm:"size:100;not null" json:"name"`
	Description string   `gorm:"size:500;not null" json:"description"`
	Builtin     bool     `json:"builtin"`
	Permissions []string `gorm:"-" json:"permissions"`
}

type IdentityGroupPermission struct {
	GroupID    int64  `gorm:"primaryKey"`
	Permission string `gorm:"primaryKey;size:64"`
}

type UserIdentityGroup struct {
	UserID  int64 `gorm:"primaryKey"`
	GroupID int64 `gorm:"primaryKey"`
}

type IdentityOperationLog struct {
	ID        int64  `gorm:"primaryKey" json:"id"`
	ActorID   int64  `json:"actor_id"`
	Action    string `gorm:"size:32" json:"action"`
	GroupID   int64  `json:"group_id"`
	UserID    int64  `json:"user_id"`
	Before    string `gorm:"column:before_json;type:text" json:"before"`
	After     string `gorm:"column:after_json;type:text" json:"after"`
	CreatedOn int64  `json:"created_on"`
}

func (u *User) HasPermission(permission string) bool {
	return u != nil && u.Status == UserStatusNormal && (u.Model == nil || u.IsDel == 0) && authz.Valid(permission) &&
		(u.IsOperator || slices.Contains(u.Permissions, permission))
}

func (u *User) PermissionList() []string {
	if u == nil || u.Status != UserStatusNormal || (u.Model != nil && u.IsDel != 0) {
		return []string{}
	}
	if u.IsOperator {
		return authz.All()
	}
	result := []string{}
	for _, p := range authz.All() {
		if slices.Contains(u.Permissions, p) {
			result = append(result, p)
		}
	}
	return result
}

func (u *User) GroupList() []IdentityGroup {
	if u == nil || u.IsOperator || u.Groups == nil {
		return []IdentityGroup{}
	}
	return slices.Clone(u.Groups)
}

// LoadUserIdentities resolves current policy in bulk, without trusting old roles.
// The caller must use fresh account fields for authorization, never cached flags.
func LoadUserIdentities(db *gorm.DB, users ...*User) error {
	ids := make([]int64, 0, len(users))
	for _, u := range users {
		if u == nil {
			continue
		}
		u.Groups, u.Permissions = []IdentityGroup{}, []string{}
		if u.Model != nil && !u.IsOperator {
			ids = append(ids, u.ID)
		}
	}
	groups := []IdentityGroup{}
	if err := db.Order("id ASC").Find(&groups).Error; err != nil {
		return err
	}
	permissions := []IdentityGroupPermission{}
	if err := db.Find(&permissions).Error; err != nil {
		return err
	}
	byGroup := make(map[int64][]string, len(groups))
	for _, p := range permissions {
		if authz.Valid(p.Permission) {
			byGroup[p.GroupID] = append(byGroup[p.GroupID], p.Permission)
		}
	}
	memberships := []UserIdentityGroup{}
	if len(ids) > 0 {
		if err := db.Where("user_id IN ?", ids).Find(&memberships).Error; err != nil {
			return err
		}
	}
	byUser := make(map[int64]map[int64]bool, len(ids))
	for _, m := range memberships {
		if byUser[m.UserID] == nil {
			byUser[m.UserID] = make(map[int64]bool)
		}
		byUser[m.UserID][m.GroupID] = true
	}
	for _, u := range users {
		if u == nil || u.IsOperator {
			continue
		}
		baseKey := "guest"
		if u.ContactVerified() {
			baseKey = "member"
		}
		seen := make(map[string]bool)
		for _, group := range groups {
			assigned := u.Model != nil && byUser[u.ID][group.ID]
			if group.Key != baseKey && (!assigned || group.Key == "guest" || group.Key == "member") {
				continue
			}
			group.Permissions = slices.Clone(byGroup[group.ID])
			if group.Permissions == nil {
				group.Permissions = []string{}
			}
			sort.Strings(group.Permissions)
			u.Groups = append(u.Groups, group)
			for _, permission := range group.Permissions {
				seen[permission] = true
			}
		}
		for permission := range seen {
			u.Permissions = append(u.Permissions, permission)
		}
		sort.Strings(u.Permissions)
	}
	return nil
}

func (u *User) ContactVerified() bool {
	if u == nil {
		return false
	}
	if conf.VerificationMode() == "phone" {
		return u.Phone != ""
	}
	return u.Email != ""
}
