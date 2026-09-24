package v1

import (
	. "github.com/alimy/mir/v5"

	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
)

// Admin 运维相关服务
type Admin struct {
	Schema `mir:"v1,chain"`

	// ChangeUserStatus 管理·禁言/解封用户
	ChangeUserStatus   func(Post, web.ChangeUserStatusReq)                            `mir:"admin/user/status"`
	SiteInfo           func(Get, web.SiteInfoReq) web.SiteInfoResp                    `mir:"admin/site/status"`
	GetSiteSettings    func(Get) web.SiteSettingsResp                                 `mir:"admin/site/profile"`
	UpdateSiteSettings func(Post, web.SiteSettingsReq) web.SiteSettingsResp           `mir:"admin/site/profile"`
	GetSettingsSchema  func(Get) web.AdminSettingsSchemaResp                          `mir:"admin/settings/schema"`
	GetSettingsValues  func(Get) web.AdminSettingsValuesResp                          `mir:"admin/settings/values"`
	SaveSettings       func(Post, web.AdminSettingsSaveReq) web.AdminSettingsSaveResp `mir:"admin/settings/save"`

	// AdminUserList 管理·用户列表搜索
	AdminUserList func(Get, web.AdminUserListReq) web.AdminUserListResp `mir:"admin/user/list"`
	// AdminUserDetail 管理·用户详情(完整手机号 管理级可见)
	AdminUserDetail func(Get, web.AdminUserDetailReq) web.AdminUserDetailResp `mir:"admin/user/detail"`
	// AdminUserRoleChange 管理·变更用户角色
	AdminUserRoleChange func(Post, web.AdminUserRoleReq) `mir:"admin/user/role"`
	// AdminUserRoleLogs 管理·角色变更记录
	AdminUserRoleLogs func(Get, web.AdminUserRoleLogsReq) web.AdminUserRoleLogsResp `mir:"admin/user/role/logs"`
	// AdminUserDelete 管理·软删除用户(标记is_del, 数据保留可恢复)
	AdminUserDelete func(Post, web.AdminUserDeleteReq) `mir:"admin/user/delete"`
}
