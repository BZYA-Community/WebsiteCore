package chain

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/pkg/app"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/gin-gonic/gin"
)

var _errPasswordChangeRequired = xerror.NewError(10404, "请先修改临时密码")

func passwordChangeRoute(c *gin.Context) bool {
	return (c.Request.Method == "GET" && c.FullPath() == "/v1/user/info") ||
		(c.Request.Method == "POST" && c.FullPath() == "/v1/user/password")
}

func CourseEditor() gin.HandlerFunc {
	return func(c *gin.Context) {
		if value, ok := c.Get("USER"); ok {
			if user, ok := value.(*ms.User); ok && (user.CanCreateCourse() || user.CanManageUsers()) {
				c.Next()
				return
			}
		}
		app.NewResponse(c).ToErrorResponse(_errNoAdminPermission)
		c.Abort()
	}
}
