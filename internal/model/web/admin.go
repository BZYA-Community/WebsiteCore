// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
)

type ChangeUserStatusReq struct {
	BaseInfo `json:"-" binding:"-"`
	ID       int64 `json:"id" form:"id" binding:"required"`
	Status   int   `json:"status" form:"status" binding:"required,oneof=1 2"`
}

type SiteInfoReq struct {
	SimpleInfo `json:"-" binding:"-"`
}

type SiteInfoResp struct {
	RegisterUserCount int64 `json:"register_user_count"`
	OnlineUserCount   int   `json:"online_user_count"`
	HistoryMaxOnline  int   `json:"history_max_online"`
	ServerUpTime      int64 `json:"server_up_time"`
}

// AdminUserListReq 用户管理·搜索用户列表(keyword匹配ID/用户名/昵称/手机号)
type AdminUserListReq struct {
	BaseInfo `form:"-" binding:"-"`
	Keyword  string `form:"keyword"`
	Page     int    `form:"-" binding:"-"`
	PageSize int    `form:"-" binding:"-"`
}

func (r *AdminUserListReq) SetPageInfo(page, pageSize int) {
	r.Page, r.PageSize = page, pageSize
}

// AdminUserItem 用户列表条目(手机号脱敏显示)
type AdminUserItem struct {
	ID        int64    `json:"id"`
	Nickname  string   `json:"nickname"`
	Username  string   `json:"username"`
	Phone     string   `json:"phone"`
	Roles     []string `json:"roles"`
	Identity  string   `json:"identity"`
	Status    int      `json:"status"`
	IsAdmin   bool     `json:"is_admin"`
	CreatedOn int64    `json:"created_on"`
}

type AdminUserListResp base.PageResp

// AdminUserDetailReq 用户管理·用户详情
type AdminUserDetailReq struct {
	BaseInfo `form:"-" binding:"-"`
	ID       int64 `form:"id" binding:"required"`
}

// AdminUserDetailResp 用户详情(管理级可见完整手机号)
type AdminUserDetailResp struct {
	ID        int64    `json:"id"`
	Nickname  string   `json:"nickname"`
	Username  string   `json:"username"`
	Phone     string   `json:"phone"`
	Roles     []string `json:"roles"`
	Identity  string   `json:"identity"`
	Status    int      `json:"status"`
	IsAdmin   bool     `json:"is_admin"`
	CreatedOn int64    `json:"created_on"`
}

// AdminUserRoleReq 用户管理·变更用户角色
type AdminUserRoleReq struct {
	BaseInfo `json:"-" binding:"-"`
	UserID   int64  `json:"user_id" binding:"required"`
	Role     string `json:"role" binding:"required"`
	Action   string `json:"action" binding:"required,oneof=add remove"`
}

// AdminUserRoleLogsReq 用户管理·角色变更记录
type AdminUserRoleLogsReq struct {
	BaseInfo `form:"-" binding:"-"`
	Page     int `form:"-" binding:"-"`
	PageSize int `form:"-" binding:"-"`
}

func (r *AdminUserRoleLogsReq) SetPageInfo(page, pageSize int) {
	r.Page, r.PageSize = page, pageSize
}

// AdminUserRoleLogItem 角色变更记录条目(用户名/操作人冗余自用户表)
type AdminUserRoleLogItem struct {
	ID           int64  `json:"id"`
	UserID       int64  `json:"user_id"`
	Username     string `json:"username"`
	OperatorID   int64  `json:"operator_id"`
	OperatorName string `json:"operator_name"`
	OldRoles     string `json:"old_roles"`
	NewRoles     string `json:"new_roles"`
	Action       string `json:"action"`
	CreatedOn    int64  `json:"created_on"`
}

type AdminUserRoleLogsResp base.PageResp

// AdminAuditPostsReq 审核队列·status: 0待审核 1已通过 2未通过 -1全部
type AdminAuditPostsReq struct {
	SimpleInfo `form:"-" binding:"-"`
	Status     int `form:"status"`
	Page       int `form:"-" binding:"-"`
	PageSize   int `form:"-" binding:"-"`
}

func (r *AdminAuditPostsReq) SetPageInfo(page, pageSize int) {
	r.Page, r.PageSize = page, pageSize
}

type AdminAuditPostsResp base.PageResp

// AdminAuditPostReq 审核动作·action: approve通过 reject拒绝(需reason) delete删除
type AdminAuditPostReq struct {
	BaseInfo `json:"-" binding:"-"`
	PostID   int64  `json:"post_id" binding:"required"`
	Action   string `json:"action" binding:"required,oneof=approve reject delete"`
	Reason   string `json:"reason"`
}

// AdminAuditLogsReq 审核日志
type AdminAuditLogsReq struct {
	SimpleInfo `form:"-" binding:"-"`
	Page       int `form:"-" binding:"-"`
	PageSize   int `form:"-" binding:"-"`
}

func (r *AdminAuditLogsReq) SetPageInfo(page, pageSize int) {
	r.Page, r.PageSize = page, pageSize
}

// AdminAuditLogItem 审核日志条目(操作人冗余自用户表)
type AdminAuditLogItem struct {
	ID           int64  `json:"id"`
	PostID       int64  `json:"post_id"`
	OperatorID   int64  `json:"operator_id"`
	OperatorName string `json:"operator_name"`
	Action       string `json:"action"`
	OldStatus    uint8  `json:"old_status"`
	NewStatus    uint8  `json:"new_status"`
	Reason       string `json:"reason"`
	CreatedOn    int64  `json:"created_on"`
}

type AdminAuditLogsResp base.PageResp
