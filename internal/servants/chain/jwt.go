// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package chain

import (
	"errors"
	"strings"

	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/pkg/app"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWT() gin.HandlerFunc {
	ums := userManageService()
	return func(c *gin.Context) {
		var (
			token string
			ecode = xerror.Success
		)
		if s, exist := c.GetQuery("token"); exist {
			token = s
		} else {
			token = c.GetHeader("Authorization")
			// 验证前端传过来的token格式，不为空，开头为Bearer
			if token == "" || !strings.HasPrefix(token, "Bearer ") {
				response := app.NewResponse(c)
				response.ToErrorResponse(xerror.UnauthorizedTokenError)
				c.Abort()
				return
			}
			// 验证通过，提取有效部分（除去Bearer)
			token = token[7:]
		}
		if token != "" {
			if claims, err := app.ParseToken(token); err == nil {
				// 加载用户信息
				if user, err := ums.GetUserByID(claims.UID); err == nil {
					// 强制下线机制
					if app.IssuerFrom(user.Salt) == claims.Issuer {
						// 状态是身份属性, 在身份解析处统一校验(#26):
						// 封禁等非正常状态的 token 一律拒绝, 不再依赖各角色门分散拦截
						if user.Status != ms.UserStatusNormal {
							ecode = _errUserHasBeenBanned
						} else if user.MustChangePassword && !passwordChangeRoute(c) {
							ecode = _errPasswordChangeRequired
						} else {
							c.Set("USER", user)
							c.Set("UID", claims.UID)
							c.Set("USERNAME", claims.Username)
						}
					} else {
						ecode = xerror.UnauthorizedTokenTimeout
					}
				} else {
					ecode = xerror.UnauthorizedAuthNotExist
				}
			} else {
				if errors.Is(err, jwt.ErrTokenExpired) {
					ecode = xerror.UnauthorizedTokenTimeout
				} else {
					ecode = xerror.UnauthorizedTokenError
				}
			}
		} else {
			ecode = xerror.InvalidParams
		}
		if ecode != xerror.Success {
			response := app.NewResponse(c)
			response.ToErrorResponse(ecode)
			c.Abort()
			return
		}
		c.Next()
	}
}

// JwtSurely enforces current account state even for lightweight count endpoints.
func JwtSurely() gin.HandlerFunc { return JWT() }

func JwtLoose() gin.HandlerFunc {
	ums := userManageService()
	return func(c *gin.Context) {
		token, exist := c.GetQuery("token")
		if !exist {
			token = c.GetHeader("Authorization")
			// 验证前端传过来的token格式，不为空，开头为Bearer
			if strings.HasPrefix(token, "Bearer ") {
				// 验证通过，提取有效部分（除去Bearer)
				token = token[7:]
			} else {
				c.Next()
			}
		}
		if len(token) > 0 {
			if claims, err := app.ParseToken(token); err == nil {
				// 加载用户信息
				user, err := ums.GetUserByID(claims.UID)
				if err == nil && app.IssuerFrom(user.Salt) == claims.Issuer {
					// 宽松链同样在解析出处强制状态校验(#26): 关注/取关等写接口
					// 也挂在 JwtLoose 上, 封禁用户不得借其继续操作;
					// 无 token 的游客不受影响(解析不出用户时保持原宽松语义)
					if user.Status != ms.UserStatusNormal {
						response := app.NewResponse(c)
						response.ToErrorResponse(_errUserHasBeenBanned)
						c.Abort()
						return
					}
					if user.MustChangePassword && !passwordChangeRoute(c) {
						app.NewResponse(c).ToErrorResponse(_errPasswordChangeRequired)
						c.Abort()
						return
					}
					c.Set("UID", claims.UID)
					c.Set("USERNAME", claims.Username)
					c.Set("USER", user)
				}
			}
		}
		c.Next()
	}
}
