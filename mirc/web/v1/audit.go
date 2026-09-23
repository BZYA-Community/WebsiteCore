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
	// ListAuditLogs 审核日志
	ListAuditLogs func(Get, web.AdminAuditLogsReq) web.AdminAuditLogsResp `mir:"admin/audit/logs"`
}
