// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package chain

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/pkg/app"
	"github.com/alimy/tryst/cfg"
	"github.com/gin-gonic/gin"
)

func Priv() gin.HandlerFunc {
	if cfg.If("PhoneBind") {
		return func(c *gin.Context) {
			if u, exist := c.Get("USER"); exist {
				if user, ok := u.(*ms.User); ok {
					if user.Status == ms.UserStatusNormal {
						// #27: 手机号唯一写入路径 UserPhoneBind 在 Sms 未启用时已整体拒绝,
						// 不再产生新的零校验绑定; 此处仍以存量手机号作为门槛——Sms 关闭的
						// 部署无任何替代验证途径, 若连存量号码也否定将全员永久卡死。
						// 残余风险: 修复前经漏洞绑定的号码仍可通过, 需运维审计清理。
						if user.Phone == "" {
							response := app.NewResponse(c)
							response.ToErrorResponse(_errAccountNoPhoneBind)
							c.Abort()
							return
						}
						c.Next()
						return
					}
				}
			}
			response := app.NewResponse(c)
			response.ToErrorResponse(_errUserHasBeenBanned)
			c.Abort()
		}
	} else {
		return func(c *gin.Context) {
			if u, exist := c.Get("USER"); exist {
				if user, ok := u.(*ms.User); ok && user.Status == ms.UserStatusNormal {
					c.Next()
					return
				}
			}
			response := app.NewResponse(c)
			response.ToErrorResponse(_errUserHasBeenBanned)
			c.Abort()
		}
	}
}
