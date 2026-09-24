// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package chain

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/pkg/app"
	"github.com/gin-gonic/gin"
)

func Admin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if user, exist := c.Get("USER"); exist {
			if userModel, ok := user.(*ms.User); ok {
				if userModel.Status == ms.UserStatusNormal && userModel.IsAdmin {
					c.Next()
					return
				}
			}
		}

		response := app.NewResponse(c)
		response.ToErrorResponse(_errNoAdminPermission)
		c.Abort()
	}
}

// Auditor 审核权限链: 管理员/运维/审核角色可访问
func Auditor() gin.HandlerFunc {
	return func(c *gin.Context) {
		if user, exist := c.Get("USER"); exist {
			if userModel, ok := user.(*ms.User); ok {
				if userModel.Status == ms.UserStatusNormal &&
					(userModel.IsAdmin || userModel.HasRole(ms.RoleAuditor)) {
					c.Next()
					return
				}
			}
		}

		response := app.NewResponse(c)
		response.ToErrorResponse(_errNoAuditPermission)
		c.Abort()
	}
}
