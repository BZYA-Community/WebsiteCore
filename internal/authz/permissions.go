// Package authz defines the permission vocabulary shared by policy and callers.
package authz

import (
	"errors"
	"slices"
)

var (
	ErrDenied     = errors.New("permission denied")
	ErrInvalid    = errors.New("invalid identity policy")
	ErrGroupInUse = errors.New("identity group still has members")
)

const (
	PostView           = "post.view"
	PostCreate         = "post.create"
	CommentCreate      = "comment.create"
	ContentUpload      = "content.upload"
	ContentReview      = "content.review"
	ContentManage      = "content.manage"
	ContentViewPrivate = "content.view_private"
	PublishUnreviewed  = "content.publish_unreviewed"
	UserManage         = "user.manage"
	IdentityManage     = "identity.manage"
	CourseCatalog      = "course.catalog"
	CourseView         = "course.view"
	CourseManage       = "course.manage"
	CourseManageOwn    = "course.manage_own"
	CourseUpload       = "course.upload"
	SiteManage         = "site.manage"
	MessageInitiate    = "message.initiate"
	MessageReply       = "message.reply"
	ProfileEdit        = "profile.edit"
	CommunityInteract  = "community.interact"
	AuditViewAll       = "audit.view_all"
)

var permissions = []string{
	PostView, PostCreate, CommentCreate, ContentUpload, ContentReview,
	ContentManage, ContentViewPrivate, PublishUnreviewed, UserManage,
	IdentityManage, CourseCatalog, CourseView, CourseManage, CourseManageOwn,
	CourseUpload, SiteManage, MessageInitiate, MessageReply, ProfileEdit,
	CommunityInteract, AuditViewAll,
}

func All() []string { return slices.Clone(permissions) }

func Valid(permission string) bool { return slices.Contains(permissions, permission) }
