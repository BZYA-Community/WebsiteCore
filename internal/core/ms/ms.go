// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package ms contain core data service interface type
// model define for gorm adapter
package ms

import (
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
)

const (
	UserStatusNormal = dbr.UserStatusNormal
	UserStatusClosed = dbr.UserStatusClosed

	RoleOperator = dbr.RoleOperator
	RoleAdmin    = dbr.RoleAdmin
	RoleAuditor  = dbr.RoleAuditor
	RoleMentor   = dbr.RoleMentor

	PostAuditPending  = dbr.PostAuditPending
	PostAuditApproved = dbr.PostAuditApproved
	PostAuditRejected = dbr.PostAuditRejected
)

// AllRoles 可由后台分配的管理角色
var AllRoles = dbr.AllRoles

type (
	User                = dbr.User
	Post                = dbr.Post
	ConditionsT         = dbr.ConditionsT
	PostFormated        = dbr.PostFormated
	UserFormated        = dbr.UserFormated
	PostContentFormated = dbr.PostContentFormated
	AuditLog            = dbr.AuditLog
	UserRoleLog         = dbr.UserRoleLog
	Model               = dbr.Model
)
