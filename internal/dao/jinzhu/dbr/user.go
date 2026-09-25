// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package dbr

import (
	"strings"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"gorm.io/gorm"
)

const (
	UserStatusNormal int = iota + 1
	UserStatusClosed
)

// 管理角色(存储于 p_user.roles 逗号分隔) 基础身份游客/道友由手机号绑定状态推导不落库
const (
	RoleOperator = "operator" // 运维
	RoleAdmin    = "admin"    // 管理员
	RoleAuditor  = "auditor"  // 审核
	RoleMentor   = "mentor"   // 导师
)

// AllRoles 可由后台分配的管理角色
var AllRoles = []string{RoleMentor, RoleAuditor, RoleAdmin, RoleOperator}

type User struct {
	*Model
	Nickname string `json:"nickname"`
	Username string `json:"username"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Salt     string `json:"salt"`
	Status   int    `json:"status"`
	Avatar   string `json:"avatar"`
	IsAdmin  bool   `json:"is_admin"`
	Roles    string `json:"roles"`
	// PendingNickname 昵称变更暂存: 提交后先存此处 审核通过才写入Nickname
	// json:"-" 避免对外泄露未审核内容
	PendingNickname string `json:"-"`
}

type UserFormated struct {
	ID          int64    `db:"id" json:"id"`
	Nickname    string   `json:"nickname"`
	Username    string   `json:"username"`
	Status      int      `json:"status"`
	Avatar      string   `json:"avatar"`
	IsAdmin     bool     `json:"is_admin"`
	Roles       []string `json:"roles"`
	Identity    string   `json:"identity"`
	IsFollowing bool     `json:"is_following"`
}

func (u *User) Format() *UserFormated {
	if u.Model != nil {
		return &UserFormated{
			ID:       u.ID,
			Nickname: u.Nickname,
			Username: u.Username,
			Status:   u.Status,
			Avatar:   u.Avatar,
			IsAdmin:  u.IsAdmin,
			Roles:    u.RoleList(),
			Identity: u.DisplayIdentity(),
		}
	}

	return nil
}

// RoleList 返回用户的管理角色列表
func (u *User) RoleList() []string {
	if u.Roles == "" {
		return []string{}
	}
	return strings.Split(u.Roles, ",")
}

// HasRole 判断用户是否拥有指定管理角色
func (u *User) HasRole(role string) bool {
	for _, r := range u.RoleList() {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole 是否拥有任意管理角色(导师/审核/管理员/运维)
func (u *User) HasAnyRole() bool {
	return u.Roles != ""
}

// IsAdminLevel 管理级身份(运维/管理员) 与 is_admin 布尔语义保持一致
func (u *User) IsAdminLevel() bool {
	return u.HasRole(RoleOperator) || u.HasRole(RoleAdmin)
}

// SyncIsAdmin 角色变更后同步 is_admin 布尔 保持既有 IsAdmin 权限触点兼容
func (u *User) SyncIsAdmin() {
	u.IsAdmin = u.IsAdminLevel()
}

// DisplayIdentity 显示身份: 运维 > 管理员 > 审核 > 导师 > 道友 > 游客
func (u *User) DisplayIdentity() string {
	return IdentityOf(u.Roles, u.Phone)
}

// IdentityOf 根据管理角色与手机号绑定状态计算显示身份
func IdentityOf(roles, phone string) string {
	u := User{Roles: roles, Phone: phone}
	switch {
	case u.HasRole(RoleOperator):
		return "运维"
	case u.HasRole(RoleAdmin):
		return "管理员"
	case u.HasRole(RoleAuditor):
		return "审核"
	case u.HasRole(RoleMentor):
		return "导师"
	case u.Phone != "":
		return "道友"
	default:
		return "游客"
	}
}

// SplitRoles 解析逗号分隔的角色串为列表
func SplitRoles(roles string) []string {
	u := User{Roles: roles}
	return u.RoleList()
}

// MaskPhone 手机号脱敏 138****1234，过短则原样返回
func MaskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

// AddRole 追加管理角色(已持有则幂等) 返回是否发生变化，角色保持 AllRoles 层级排序
func (u *User) AddRole(role string) bool {
	if u.HasRole(role) {
		return false
	}
	u.Roles = strings.Join(sortRoles(append(u.RoleList(), role)), ",")
	return true
}

// RemoveRole 移除管理角色(未持有则幂等) 返回是否发生变化
func (u *User) RemoveRole(role string) bool {
	roles := u.RoleList()
	res := make([]string, 0, len(roles))
	found := false
	for _, r := range roles {
		if r == role {
			found = true
			continue
		}
		res = append(res, r)
	}
	if !found {
		return false
	}
	u.Roles = strings.Join(res, ",")
	return true
}

// sortRoles 按角色等级排序便于稳定展示
func sortRoles(roles []string) []string {
	res := make([]string, 0, len(roles))
	for _, ar := range AllRoles {
		for _, r := range roles {
			if r == ar {
				res = append(res, r)
				break
			}
		}
	}
	return res
}

func (u *User) Get(db *gorm.DB) (*User, error) {
	var user User
	if u.Model != nil && u.Model.ID > 0 {
		db = db.Where("id= ? AND is_del = ?", u.Model.ID, 0)
	} else if u.Phone != "" {
		db = db.Where("phone = ? AND is_del = ?", u.Phone, 0)
	} else {
		db = db.Where("username = ? AND is_del = ?", u.Username, 0)
	}

	err := db.First(&user).Error
	if err != nil {
		return &user, err
	}

	return &user, nil
}

func (u *User) List(db *gorm.DB, conditions *ConditionsT, offset, limit int) ([]*User, error) {
	var users []*User
	var err error
	if offset >= 0 && limit > 0 {
		db = db.Offset(offset).Limit(limit)
	}
	for k, v := range *conditions {
		if k == "ORDER" {
			db = db.Order(v)
		} else {
			db = db.Where(k, v)
		}
	}

	if err = db.Where("is_del = ?", 0).Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}

func (u *User) ListUserInfoById(db *gorm.DB, ids []int64) (res cs.UserInfoList, err error) {
	err = db.Model(u).Where("id IN ?", ids).Find(&res).Error
	return
}

func (u *User) Create(db *gorm.DB) (*User, error) {
	err := db.Create(&u).Error
	return u, err
}

func (u *User) Update(db *gorm.DB) error {
	return db.Model(&User{}).Where("id = ? AND is_del = ?", u.Model.ID, 0).Save(u).Error
}

// Delete 软删除用户(标记is_del=1): 无法登录/查询, 数据保留可恢复
func (u *User) Delete(db *gorm.DB) error {
	return db.Model(u).Where("id = ?", u.Model.ID).Updates(map[string]any{
		"deleted_on": time.Now().Unix(),
		"is_del":     1,
	}).Error
}
