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

// EligibleReviewers selects one row per user who can review under the current policy.
func EligibleReviewers(db *gorm.DB) *gorm.DB {
	activeUsers := func() *gorm.DB {
		return db.Model(&User{}).Select("id").Where("is_del = 0 AND status = ?", UserStatusNormal)
	}
	reviewGrants := func(groups *gorm.DB) *gorm.DB {
		return db.Model(&IdentityGroupPermission{}).Select("group_id").
			Where("permission = ? AND group_id IN (?)", authz.ContentReview, groups)
	}
	groups := db.Model(&IdentityGroup{}).Select("id").Session(&gorm.Session{})
	assigned := db.Model(&UserIdentityGroup{}).Distinct("user_id").
		Where("group_id IN (?)", reviewGrants(groups.Where("key NOT IN ?", []string{"guest", "member"})))
	// LIMIT keeps account lookups correlated with matching memberships, so
	// PostgreSQL can use the user primary key as the account table grows.
	assignedReviewers := db.Table("(?) AS assigned", assigned).Select("eligible.id").
		Joins("JOIN LATERAL (?) AS eligible ON true", activeUsers().Where("id = assigned.user_id").Limit(1))
	guestGrant := reviewGrants(groups.Where("key = ?", "guest"))
	memberGrant := reviewGrants(groups.Where("key = ?", "member"))
	// Both contact columns are NOT NULL. Use the direct predicate so PostgreSQL
	// can use the partial index for active accounts with a verified contact.
	contact := "email"
	if conf.VerificationMode() == "phone" {
		contact = "phone"
	}

	// Automatic groups can grant review without a membership row. UNION keeps
	// overlapping grants from giving any user extra weight in random selection.
	candidates := db.Raw("? UNION ? UNION ? UNION ?",
		activeUsers().Where("is_operator"),
		assignedReviewers,
		activeUsers().Where("EXISTS (?) AND "+contact+" = ''", guestGrant),
		activeUsers().Where("EXISTS (?) AND "+contact+" <> ''", memberGrant),
	)
	return db.Table("(?) AS reviewers", candidates)
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
