// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style license.

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

const (
	RoleOperator  = "operator"
	RoleAdmin     = "admin"
	RoleAuditor   = "auditor"
	MemberStudent = "student"
	MemberTeacher = "teacher"
)

var AllRoles = []string{RoleAuditor, RoleAdmin, RoleOperator}

type User struct {
	*Model
	Nickname string `json:"nickname"`
	Username string `json:"username"`
	Phone    string `json:"phone"`
	Password string `json:"-"`
	Salt     string `json:"-"`
	Status   int    `json:"status"`
	Avatar   string `json:"avatar"`
	Roles    string `json:"-"`
	// AccountType is immutable. A disabled Admin never becomes a member.
	AccountType        string  `json:"-"`
	MemberIdentity     *string `json:"member_identity"`
	IsMentor           bool    `json:"is_mentor"`
	MustChangePassword bool    `json:"-"`
	PendingNickname    string  `json:"-"`
	PendingAvatar      string  `json:"-"`
}

type UserFormated struct {
	ID             int64    `db:"id" json:"id"`
	Nickname       string   `json:"nickname"`
	Username       string   `json:"username"`
	Status         int      `json:"status"`
	Avatar         string   `json:"avatar"`
	Roles          []string `json:"roles"`
	MemberIdentity *string  `json:"member_identity"`
	IsMentor       bool     `json:"is_mentor"`
	IsFollowing    bool     `json:"is_following"`
}

// Format is a public projection: Auditor is private even for content authors.
func (u *User) Format() *UserFormated {
	if u == nil || u.Model == nil {
		return nil
	}
	return &UserFormated{
		ID: u.ID, Nickname: u.Nickname, Username: u.Username,
		Status: u.Status, Avatar: u.Avatar, Roles: u.PublicRoles(),
		MemberIdentity: u.MemberIdentity, IsMentor: u.IsMentor,
	}
}

func (u *User) RoleList() []string {
	if u == nil || u.Roles == "" {
		return []string{}
	}
	return strings.Split(u.Roles, ",")
}

func (u *User) HasRole(role string) bool {
	if u == nil {
		return false
	}
	for _, r := range u.RoleList() {
		if r == role {
			return true
		}
	}
	return false
}

func PublicRoles(roles string) []string {
	switch roles {
	case RoleOperator, RoleAdmin:
		return []string{roles}
	default:
		return []string{}
	}
}

func (u *User) PublicRoles() []string { return PublicRoles(u.Roles) }

func (u *User) IsActive() bool {
	return u != nil && u.Model != nil && u.ID > 0 && u.IsDel == 0 && u.Status == UserStatusNormal
}

func (u *User) IsAdminLevel() bool {
	return u != nil && u.MemberIdentity == nil && (u.Roles == RoleOperator || u.Roles == RoleAdmin)
}

func (u *User) IsTeacher() bool {
	return u != nil && u.MemberIdentity != nil && *u.MemberIdentity == MemberTeacher && !u.IsAdminLevel()
}

func (u *User) IsStudent() bool {
	return u != nil && u.MemberIdentity != nil && *u.MemberIdentity == MemberStudent && !u.IsAdminLevel()
}

func (u *User) CanManageUsers() bool  { return u.IsActive() && u.IsAdminLevel() }
func (u *User) CanCreateAdmin() bool  { return u.CanManageUsers() && u.Roles == RoleOperator }
func (u *User) CanManageAdmins() bool { return u.CanManageUsers() }
func (u *User) CanAudit() bool {
	return u.IsActive() && (u.IsAdminLevel() || u.HasRole(RoleAuditor))
}

func (u *User) CanAuditUser(authorID int64) bool {
	return u.CanAudit() && u.ID != authorID
}

func (u *User) CanViewSystemInfo() bool {
	return u.CanManageUsers() && u.Roles == RoleOperator
}

func (u *User) CanPublishDirectly() bool {
	return u.IsActive() && u.IsAdminLevel()
}

func (u *User) CanCreateCourse() bool {
	return u.IsActive() && (u.IsTeacher() || u.IsAdminLevel())
}

func (u *User) CanEditCourse(teacherID int64) bool {
	return u.IsActive() && (u.IsAdminLevel() || (u.IsTeacher() && u.ID == teacherID))
}
func (u *User) CanDeleteCourse() bool { return u.CanManageUsers() }

// ValidIdentity mirrors the database check; missing identities never imply a visitor.
func (u *User) ValidIdentity() bool {
	if u == nil {
		return false
	}
	switch u.AccountType {
	case "member":
		return (u.IsStudent() || u.IsTeacher()) && (u.Roles == "" || u.Roles == RoleAuditor)
	case "operator":
		return u.MemberIdentity == nil && !u.IsMentor && u.Roles == RoleOperator
	case "admin":
		return u.MemberIdentity == nil && !u.IsMentor && (u.Roles == RoleAdmin || (u.Roles == "" && u.Status == UserStatusClosed))
	default:
		return false
	}
}

func SplitRoles(roles string) []string { return (&User{Roles: roles}).RoleList() }

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

// Update writes only the submitted profile fields. A stale nickname request must
// never restore an old password, first-login flag, or administrative privileges.
func (u *User) Update(db *gorm.DB, fields ...string) error {
	if len(fields) == 0 {
		return ErrPermission
	}
	for _, field := range fields {
		switch field {
		case "nickname", "phone", "password", "salt", "avatar", "pending_nickname", "pending_avatar", "must_change_password":
		default:
			return ErrPermission
		}
	}
	return db.Model(&User{}).Where("id = ? AND is_del = ?", u.ID, 0).Select(fields).Updates(u).Error
}

// Delete 软删除用户(标记is_del=1): 无法登录/查询, 数据保留可恢复
func (u *User) Delete(db *gorm.DB) error {
	return db.Model(u).Where("id = ?", u.Model.ID).Updates(map[string]any{
		"deleted_on": time.Now().Unix(),
		"is_del":     1,
	}).Error
}
