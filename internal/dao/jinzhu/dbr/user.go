// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package dbr

import (
	"fmt"
	"slices"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/core/cs"
	"gorm.io/gorm"
)

const (
	UserStatusNormal int = iota + 1
	UserStatusClosed
)

type User struct {
	*Model
	Nickname    string          `json:"nickname"`
	Username    string          `json:"username"`
	Phone       string          `json:"phone"`
	Email       string          `json:"email"`
	Password    string          `json:"password"`
	Salt        string          `json:"salt"`
	Status      int             `json:"status"`
	Avatar      string          `json:"avatar"`
	IsAdmin     bool            `json:"is_admin"`
	Roles       string          `json:"roles"`
	IsOperator  bool            `json:"is_operator"`
	Groups      []IdentityGroup `gorm:"-" json:"groups"`
	Permissions []string        `gorm:"-" json:"permissions"`
	// PendingNickname 昵称变更暂存: 提交后先存此处 审核通过才写入Nickname
	// json:"-" 避免对外泄露未审核内容
	PendingNickname string `json:"-"`
	// PendingAvatar 头像变更暂存: 提交后先存此处 审核通过才写入Avatar
	// json:"-" 避免对外泄露未审核内容
	PendingAvatar string `json:"-"`
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
	IsOperator  bool     `json:"is_operator"`
	IsFollowing bool     `json:"is_following"`
}

func (u *User) Format() *UserFormated {
	if u.Model != nil {
		return &UserFormated{
			ID:         u.ID,
			Nickname:   u.Nickname,
			Username:   u.Username,
			Status:     u.Status,
			Avatar:     u.Avatar,
			IsAdmin:    u.HasPermission("user.manage"),
			Roles:      u.RoleList(),
			Identity:   u.DisplayIdentity(),
			IsOperator: u.IsOperator,
		}
	}

	return nil
}

// RoleList keeps the old display field populated from current identity groups.
func (u *User) RoleList() []string {
	keys := []string{}
	for _, group := range u.GroupList() {
		keys = append(keys, group.Key)
	}
	return keys
}

// DisplayIdentity returns a display label, never an authorization decision.
func (u *User) DisplayIdentity() string {
	if u.IsOperator {
		return "运维"
	}
	for _, group := range u.Groups {
		if group.Key != "guest" && group.Key != "member" {
			return group.Name
		}
	}
	for _, group := range u.Groups {
		if group.Key == "member" {
			return group.Name
		}
	}
	return "游客"
}

// MaskPhone 手机号脱敏 138****1234，过短则原样返回
func MaskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
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

	return &user, LoadUserIdentities(db.Session(&gorm.Session{NewDB: true}), &user)
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

	return users, LoadUserIdentities(db.Session(&gorm.Session{NewDB: true}), users...)
}

func (u *User) ListUserInfoById(db *gorm.DB, ids []int64) (res cs.UserInfoList, err error) {
	err = db.Model(u).Where("id IN ?", ids).Find(&res).Error
	return
}

func (u *User) Create(db *gorm.DB) (*User, error) {
	err := db.Create(&u).Error
	return u, err
}

func (u *User) Update(db *gorm.DB, fields ...string) error {
	if u.Model == nil || u.ID <= 0 || len(fields) == 0 {
		return fmt.Errorf("profile update requires a user and explicit fields")
	}
	allowed := []string{"phone", "password", "salt", "nickname", "avatar", "pending_nickname", "pending_avatar"}
	for _, field := range fields {
		if !slices.Contains(allowed, field) {
			return fmt.Errorf("invalid profile field %q", field)
		}
	}
	// Never save a stale account or credential snapshot with an unrelated edit.
	return db.Model(&User{}).Where("id = ? AND is_del = ?", u.ID, 0).
		Select(append(slices.Clone(fields), "modified_on")).Updates(u).Error
}

// Delete 软删除用户(标记is_del=1): 无法登录/查询, 数据保留可恢复
func (u *User) Delete(db *gorm.DB) error {
	return db.Model(u).Where("id = ?", u.Model.ID).Updates(map[string]any{
		"deleted_on": time.Now().Unix(),
		"is_del":     1,
	}).Error
}
