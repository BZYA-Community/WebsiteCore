// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package core

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
)

// SiteAdminService 站点管理服务(后台用户管理/内容审核/角色与审核日志)
type SiteAdminService interface {
	// 用户管理: 按关键词(ID/用户名/昵称/手机号)搜索用户
	GetUsersByAdminQuery(keyword string, offset, limit int) ([]*ms.User, int64, error)
	// 用户管理: 软删除用户(is_del=1, 无法登录/前台消失, 数据保留可恢复)
	SoftDeleteUser(user *ms.User) error
	// 审核队列: status为审核状态 -1表示全部 仅含未软删帖子
	ListAuditPosts(status int, offset, limit int) ([]*ms.Post, int64, error)
	// 审核操作日志
	CreateAuditLog(log *ms.AuditLog) error
	ListAuditLogs(offset, limit int) ([]*ms.AuditLog, int64, error)
	// 用户角色变更日志
	CreateUserRoleLog(log *ms.UserRoleLog) error
	ListUserRoleLogs(offset, limit int) ([]*ms.UserRoleLog, int64, error)
}
