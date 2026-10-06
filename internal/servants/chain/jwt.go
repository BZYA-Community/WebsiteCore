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

func JWT() gin.HandlerFunc      { return authenticate(true) }
func JwtLoose() gin.HandlerFunc { return authenticate(false) }

func authenticate(required bool) gin.HandlerFunc {
	users := userManageService()
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" && !required {
			c.Next()
			return
		}
		fail := func(err *xerror.Error) {
			app.NewResponse(c).ToErrorResponse(err)
			c.Abort()
		}
		if !strings.HasPrefix(header, "Bearer ") || len(header) == len("Bearer ") {
			fail(xerror.UnauthorizedTokenError)
			return
		}
		claims, err := app.ParseToken(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				fail(xerror.UnauthorizedTokenTimeout)
			} else {
				fail(xerror.UnauthorizedTokenError)
			}
			return
		}
		user, err := users.GetUserByID(claims.UID)
		if err != nil || user == nil || user.Model == nil {
			fail(xerror.UnauthorizedAuthNotExist)
			return
		}
		if user.Status != ms.UserStatusNormal {
			fail(_errUserHasBeenBanned)
			return
		}
		if app.IssuerFrom(user.Salt) != claims.Issuer {
			fail(xerror.UnauthorizedTokenTimeout)
			return
		}
		c.Set("USER", user)
		c.Set("UID", user.ID)
		c.Set("USERNAME", user.Username)
		c.Next()
	}
}
