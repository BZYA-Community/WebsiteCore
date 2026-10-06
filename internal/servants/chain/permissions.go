package chain

import (
	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/pkg/app"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/gin-gonic/gin"
)

type routeRule struct {
	All   []string
	Any   []string
	Login bool
}

// Every protected route is explicit; a new route without a policy fails closed.
var routeRules = func() map[string]routeRule {
	rules := make(map[string]routeRule)
	add := func(permission string, login bool, routes ...string) {
		for _, route := range routes {
			rules[route] = routeRule{All: []string{permission}, Login: login}
		}
	}
	add(authz.PostView, false, "GET /v1/posts", "GET /v1/user/posts", "GET /v1/user/profile", "GET /v1/tags", "GET /v1/post/comments", "GET /v1/post", "GET /v1/user/follows", "GET /v1/user/followings")
	add(authz.PostView, true, "GET /v1/attachment", "GET /v1/user/collections", "GET /v1/user/stars", "GET /v1/post/star", "GET /v1/post/collection", "GET /v1/suggest/tags", "GET /v1/suggest/users", "GET /v1/trends/index")
	add(authz.ProfileEdit, true, "GET /v1/user/info", "GET /v1/user/messages", "POST /v1/user/message/read", "POST /v1/user/message/readall", "POST /v1/user/phone", "POST /v1/user/password", "POST /v1/user/nickname", "POST /v1/user/avatar", "GET /v1/user/msgcount/unread", "GET /v1/user/chat/contacts", "GET /v1/user/chat/history")
	// Session introspection remains available even when all feature grants are revoked.
	rules["GET /v1/user/info"] = routeRule{Login: true}
	add(authz.PostCreate, true, "POST /v1/post", "POST /v1/post/visibility")
	add(authz.CommentCreate, true, "POST /v1/post/comment", "POST /v1/post/comment/reply")
	add(authz.ContentUpload, true, "POST /v1/attachment")
	add(authz.CommunityInteract, true, "POST /v1/post/star", "POST /v1/post/collection", "POST /v1/tweet/comment/thumbsup", "POST /v1/tweet/comment/thumbsdown", "POST /v1/tweet/reply/thumbsup", "POST /v1/tweet/reply/thumbsdown", "POST /v1/topic/follow", "POST /v1/topic/unfollow", "POST /v1/user/follow", "POST /v1/user/unfollow")
	add(authz.ContentManage, true, "POST /v1/post/stick", "POST /v1/post/highlight", "POST /v1/topic/stick", "POST /v1/topic/pin")
	for _, path := range []string{"DELETE /v1/post", "DELETE /v1/post/comment", "DELETE /v1/post/comment/reply", "POST /v1/post/lock", "POST /v1/post/comment/highlight"} {
		rules[path] = routeRule{Any: []string{authz.CommunityInteract, authz.ContentManage}, Login: true}
	}
	add(authz.CourseCatalog, false, "GET /v1/course/groups", "GET /v1/course/list")
	add(authz.CourseView, false, "GET /v1/course", "GET /v1/course/comments", "GET /v1/course/video", "POST /v1/course/play")
	for _, path := range []string{"POST /v1/course/comment", "POST /v1/course/comment/reply"} {
		rules[path] = routeRule{All: []string{authz.CourseView, authz.CommentCreate}, Login: true}
	}
	for _, path := range []string{"DELETE /v1/course/comment", "DELETE /v1/course/comment/reply"} {
		rules[path] = routeRule{All: []string{authz.CourseView}, Any: []string{authz.CommunityInteract, authz.ContentManage}, Login: true}
	}
	add(authz.CourseManage, true, "POST /v1/admin/course/group", "POST /v1/admin/course/group/update", "POST /v1/admin/course/group/delete")
	for _, path := range []string{"POST /v1/admin/course", "POST /v1/admin/course/update", "POST /v1/admin/course/delete"} {
		rules[path] = routeRule{Any: []string{authz.CourseManage, authz.CourseManageOwn}, Login: true}
	}
	for _, path := range []string{"GET /v1/admin/course/upload-credential", "POST /v1/admin/course/video"} {
		rules[path] = routeRule{All: []string{authz.CourseUpload}, Any: []string{authz.CourseManage, authz.CourseManageOwn}, Login: true}
	}
	add(authz.UserManage, true, "POST /v1/admin/user/status", "GET /v1/admin/user/list", "GET /v1/admin/user/detail", "POST /v1/admin/user/delete")
	add(authz.SiteManage, true, "GET /v1/admin/site/status", "GET /v1/admin/site/profile", "POST /v1/admin/site/profile", "GET /v1/admin/settings/schema", "GET /v1/admin/settings/values", "POST /v1/admin/settings/save", "GET /v1/sync/index")
	add(authz.IdentityManage, true, "POST /v1/admin/identity/groups", "DELETE /v1/admin/identity/groups", "GET /v1/admin/identity/permissions", "GET /v1/admin/identity/logs", "POST /v1/admin/user/identity")
	rules["GET /v1/admin/identity/groups"] = routeRule{Any: []string{authz.UserManage, authz.IdentityManage}, Login: true}
	rules["POST /v1/user/chat/send"] = routeRule{Any: []string{authz.MessageInitiate, authz.MessageReply}, Login: true}
	add(authz.ContentReview, true, "GET /v1/admin/audit/posts", "POST /v1/admin/audit/post", "GET /v1/admin/audit/comments", "POST /v1/admin/audit/comment", "GET /v1/admin/audit/nicknames", "POST /v1/admin/audit/nickname", "GET /v1/admin/audit/avatars", "POST /v1/admin/audit/avatar", "GET /v1/admin/audit/logs")
	return rules
}()

func allowedByRule(user *ms.User, loggedIn bool, rule routeRule) bool {
	if rule.Login && !loggedIn {
		return false
	}
	for _, permission := range rule.All {
		if !user.HasPermission(permission) {
			return false
		}
	}
	if len(rule.Any) == 0 {
		return true
	}
	for _, permission := range rule.Any {
		if user.HasPermission(permission) {
			return true
		}
	}
	return false
}

func Authorize() gin.HandlerFunc {
	return authorizeWith(func(users ...*ms.User) error { return userManageService().LoadUserIdentities(users...) })
}

func authorizeWith(load func(...*ms.User) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		rule, exists := routeRules[c.Request.Method+" "+c.FullPath()]
		value, _ := c.Get("USER")
		user, _ := value.(*ms.User)
		loggedIn := user != nil && user.Model != nil && user.ID > 0
		if exists && user == nil && !rule.Login {
			user = &ms.User{Status: ms.UserStatusNormal}
			if err := load(user); err != nil {
				app.NewResponse(c).ToErrorResponse(xerror.ServerError)
				c.Abort()
				return
			}
		}
		if !exists || !allowedByRule(user, loggedIn, rule) {
			app.NewResponse(c).ToErrorResponse(xerror.NewError(20007, "无权限执行该请求"))
			c.Abort()
			return
		}
		c.Next()
	}
}
