package v1

import (
	. "github.com/alimy/mir/v5"

	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
)

// Audit 内容审核服务(管理员/运维/审核角色可访问)
type Audit struct {
	Schema `mir:"v1,chain"`

	// ListAuditPosts 审核队列·按状态筛选
	ListAuditPosts func(Get, web.AdminAuditPostsReq) web.AdminAuditPostsResp `mir:"admin/audit/posts"`
	// AuditPostAction 审核动作·通过/拒绝/删除
	AuditPostAction func(Post, web.AdminAuditPostReq) `mir:"admin/audit/post"`
	// ListAuditComments 评论审核队列·评论与回复合并按状态筛选
	ListAuditComments func(Get, web.AdminAuditCommentsReq) web.AdminAuditCommentsResp `mir:"admin/audit/comments"`
	// AuditCommentAction 评论审核动作·通过/拒绝评论或回复
	AuditCommentAction func(Post, web.AdminAuditCommentReq) `mir:"admin/audit/comment"`
	// ListAuditNicknames 昵称审核队列
	ListAuditNicknames func(Get, web.AdminAuditNicknamesReq) web.AdminAuditNicknamesResp `mir:"admin/audit/nicknames"`
	// AuditNicknameAction 昵称审核动作·通过/拒绝昵称变更
	AuditNicknameAction func(Post, web.AdminAuditNicknameReq) `mir:"admin/audit/nickname"`
	// ListAuditAvatars 头像审核队列
	ListAuditAvatars func(Get, web.AdminAuditAvatarsReq) web.AdminAuditAvatarsResp `mir:"admin/audit/avatars"`
	// AuditAvatarAction 头像审核动作·通过/拒绝头像变更
	AuditAvatarAction func(Post, web.AdminAuditAvatarReq) `mir:"admin/audit/avatar"`
	// ListAuditLogs 审核日志
	ListAuditLogs func(Get, web.AdminAuditLogsReq) web.AdminAuditLogsResp `mir:"admin/audit/logs"`
}
