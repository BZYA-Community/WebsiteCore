package dbr

import (
	"maps"
	"slices"

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

type applicableIdentityGroup struct {
	IdentityGroup
	AssignedUserIDs []int64  `gorm:"serializer:json"`
	Grants          []string `gorm:"serializer:json"`
}

func findApplicableIdentityGroups(db *gorm.DB, baseKeys []string, userIDs []int64) ([]applicableIdentityGroup, error) {
	groups := db.Model(&IdentityGroup{}).Select("*, NULL::json AS assigned_user_ids").Where("key IN ?", baseKeys)
	if len(userIDs) > 0 {
		memberships := db.Model(&UserIdentityGroup{}).
			Select("group_id, json_agg(user_id) AS assigned_user_ids").
			Where("user_id IN ?", userIDs).Group("group_id")
		assigned := db.Table("(?) AS policy_group", db.Model(&IdentityGroup{})).
			Select("policy_group.*, memberships.assigned_user_ids").
			Joins("JOIN (?) AS memberships ON memberships.group_id = policy_group.id", memberships).
			Where("policy_group.key NOT IN ?", []string{"guest", "member"})
		groups = db.Raw("? UNION ALL ?", groups, assigned)
	}

	// Aggregate only matching grants and keep one row per applicable group,
	// even when a group has no grants or is shared by several users in the batch.
	grants := db.Model(&IdentityGroupPermission{}).Select("json_agg(permission)").Where("group_id = applicable.id")
	var result []applicableIdentityGroup
	err := db.Table("(?) AS applicable", groups).
		Select("applicable.*, (?) AS grants", grants).Order("applicable.id ASC").Find(&result).Error
	return result, err
}

// LoadUserIdentities resolves current policy in bulk, without trusting old roles.
// The caller must use fresh account fields for authorization, never cached flags.
func LoadUserIdentities(db *gorm.DB, users ...*User) error {
	subjects := make(map[*User]struct{}, len(users))
	for _, u := range users {
		if u == nil {
			continue
		}
		u.Groups, u.Permissions = []IdentityGroup{}, []string{}
		if !u.IsOperator {
			subjects[u] = struct{}{}
		}
	}
	if len(subjects) == 0 {
		return nil
	}

	byBaseKey := make(map[string][]*User, 2)
	byUserID := make(map[int64][]*User, len(subjects))
	for u := range subjects {
		baseKey := "guest"
		if u.ContactVerified() {
			baseKey = "member"
		}
		byBaseKey[baseKey] = append(byBaseKey[baseKey], u)
		if u.Model != nil {
			byUserID[u.ID] = append(byUserID[u.ID], u)
		}
	}

	groups, err := findApplicableIdentityGroups(db, slices.Collect(maps.Keys(byBaseKey)), slices.Collect(maps.Keys(byUserID)))
	if err != nil {
		return err
	}
	for _, policy := range groups {
		group := policy.IdentityGroup
		group.Permissions = make([]string, 0, len(policy.Grants))
		for _, permission := range policy.Grants {
			if authz.Valid(permission) {
				group.Permissions = append(group.Permissions, permission)
			}
		}
		slices.Sort(group.Permissions)

		recipients := byBaseKey[group.Key]
		for _, userID := range policy.AssignedUserIDs {
			recipients = append(recipients, byUserID[userID]...)
		}
		for _, u := range recipients {
			resolved := group
			resolved.Permissions = slices.Clone(group.Permissions)
			u.Groups = append(u.Groups, resolved)
			u.Permissions = append(u.Permissions, group.Permissions...)
		}
	}
	for u := range subjects {
		slices.Sort(u.Permissions)
		u.Permissions = slices.Compact(u.Permissions)
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
