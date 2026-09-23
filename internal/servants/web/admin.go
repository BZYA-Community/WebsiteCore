// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	api "github.com/BZYA-Community/WebsiteCore/auto/api/v1"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/chain"
	"github.com/BZYA-Community/WebsiteCore/internal/sitesetting"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/sirupsen/logrus"
)

type adminSrv struct {
	api.UnimplementedAdminServant
	*base.DaoServant
	wc           core.WebCache
	settings     *sitesetting.Service
	serverUpTime int64
}

func (s *adminSrv) Chain() gin.HandlersChain {
	return gin.HandlersChain{chain.JWT(), chain.Admin()}
}

func (s *adminSrv) ChangeUserStatus(req *web.ChangeUserStatusReq) error {
	user, err := s.Ds.GetUserByID(req.ID)
	if err != nil || user.Model == nil || user.ID <= 0 {
		return web.ErrNoExistUsername
	}
	// 执行更新
	user.Status = req.Status
	if err := s.Ds.UpdateUser(user); err != nil {
		return xerror.ServerError
	}
	return nil
}

func (s *adminSrv) SiteInfo(req *web.SiteInfoReq) (*web.SiteInfoResp, error) {
	res, err := &web.SiteInfoResp{ServerUpTime: s.serverUpTime}, error(nil)
	res.RegisterUserCount, err = s.Ds.GetRegisterUserCount()
	if err != nil {
		logrus.Errorf("get SiteInfo[1] occurs error: %s", err)
	}
	onlineUserKeys, xerr := s.wc.Keys(conf.PrefixOnlineUser + "*")
	if xerr == nil {
		res.OnlineUserCount = len(onlineUserKeys)
		if res.HistoryMaxOnline, err = s.wc.PutHistoryMaxOnline(res.OnlineUserCount); err != nil {
			logrus.Errorf("get Siteinfo[3] occurs error: %s", err)
		}
	} else {
		logrus.Errorf("get Siteinfo[2] occurs error: %s", err)
	}
	// 错误进行宽松赦免处理
	return res, nil
}

func (s *adminSrv) GetSiteSettings() (*web.SiteSettingsResp, error) {
	profile, err := s.settings.GetProfile(context.Background())
	if err != nil {
		return nil, err
	}
	return &web.SiteSettingsResp{
		SiteProfileResp: *profile,
		ReadonlyFields:  sitesetting.CloneReadonlyFields(),
	}, nil
}

func (s *adminSrv) UpdateSiteSettings(req *web.SiteSettingsReq) (*web.SiteSettingsResp, error) {
	profile, err := s.settings.UpdateEditableProfile(context.Background(), sitesetting.EditableFromRequest(req))
	if err != nil {
		return nil, err
	}
	return &web.SiteSettingsResp{
		SiteProfileResp: *profile,
		ReadonlyFields:  sitesetting.CloneReadonlyFields(),
	}, nil
}

func (s *adminSrv) GetSettingsSchema() (*web.AdminSettingsSchemaResp, error) {
	return s.settings.GetSchema()
}

func (s *adminSrv) GetSettingsValues() (*web.AdminSettingsValuesResp, error) {
	return s.settings.GetValues(context.Background())
}

func (s *adminSrv) SaveSettings(req *web.AdminSettingsSaveReq) (*web.AdminSettingsSaveResp, error) {
	return s.settings.SaveValues(context.Background(), req.Items)
}

// maskPhone 手机号脱敏 138****1234
func maskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

// AdminUserList 用户管理·搜索用户列表
func (s *adminSrv) AdminUserList(req *web.AdminUserListReq) (*web.AdminUserListResp, error) {
	limit, offset := req.PageSize, (req.Page-1)*req.PageSize
	users, total, err := s.Ds.GetUsersByAdminQuery(req.Keyword, offset, limit)
	if err != nil {
		logrus.Errorf("Ds.GetUsersByAdminQuery err: %s", err)
		return nil, web.ErrGetPostsFailed
	}
	items := make([]*web.AdminUserItem, 0, len(users))
	for _, user := range users {
		items = append(items, &web.AdminUserItem{
			ID:        user.ID,
			Nickname:  user.Nickname,
			Username:  user.Username,
			Phone:     maskPhone(user.Phone),
			Roles:     user.RoleList(),
			Identity:  user.DisplayIdentity(),
			Status:    user.Status,
			IsAdmin:   user.IsAdmin,
			CreatedOn: user.CreatedOn,
		})
	}
	return (*web.AdminUserListResp)(base.PageRespFrom(items, req.Page, req.PageSize, total)), nil
}

// AdminUserDetail 用户管理·用户详情(完整手机号)
func (s *adminSrv) AdminUserDetail(req *web.AdminUserDetailReq) (*web.AdminUserDetailResp, error) {
	user, err := s.Ds.GetUserByID(req.ID)
	if err != nil || user.Model == nil || user.ID <= 0 {
		return nil, web.ErrNoExistUsername
	}
	return &web.AdminUserDetailResp{
		ID:        user.ID,
		Nickname:  user.Nickname,
		Username:  user.Username,
		Phone:     user.Phone,
		Roles:     user.RoleList(),
		Identity:  user.DisplayIdentity(),
		Status:    user.Status,
		IsAdmin:   user.IsAdmin,
		CreatedOn: user.CreatedOn,
	}, nil
}

// AdminUserRoleChange 用户管理·变更用户角色
// 权限规则: 运维可管理所有角色; 管理员只能管理 mentor/auditor/admin 且不可变更运维账号
func (s *adminSrv) AdminUserRoleChange(req *web.AdminUserRoleReq) error {
	if req.User == nil {
		return web.ErrNoPermission
	}
	valid := false
	for _, r := range ms.AllRoles {
		if req.Role == r {
			valid = true
			break
		}
	}
	if !valid {
		return xerror.InvalidParams.WithDetails("无效的管理角色: " + req.Role)
	}
	user, err := s.Ds.GetUserByID(req.UserID)
	if err != nil || user.Model == nil || user.ID <= 0 {
		return web.ErrNoExistUsername
	}
	// operator相关变更(授予/移除operator角色 或 修改运维账号)仅运维可操作
	if req.Role == ms.RoleOperator || user.HasRole(ms.RoleOperator) {
		if !req.User.HasRole(ms.RoleOperator) {
			return web.ErrRoleChangeNoPermission
		}
	}
	oldRoles := user.Roles
	if req.Action == "add" {
		user.AddRole(req.Role)
	} else {
		user.RemoveRole(req.Role)
	}
	if user.Roles == oldRoles {
		// 幂等: 无变化直接成功
		return nil
	}
	user.SyncIsAdmin()
	if err := s.Ds.UpdateUser(user); err != nil {
		logrus.Errorf("Ds.UpdateUser err: %s", err)
		return web.ErrRoleChangeFailed
	}
	// 写角色变更日志(宽松处理错误)
	if err := s.Ds.CreateUserRoleLog(&ms.UserRoleLog{
		UserID:     user.ID,
		OperatorID: req.User.ID,
		OldRoles:   oldRoles,
		NewRoles:   user.Roles,
		Action:     req.Action,
	}); err != nil {
		logrus.Errorf("Ds.CreateUserRoleLog err: %s", err)
	}
	// 过期该用户缓存(info:id/info:name/profile:name)
	onChangeUsernameEvent(user.ID, user.Username)
	return nil
}

// AdminUserRoleLogs 用户管理·角色变更记录
func (s *adminSrv) AdminUserRoleLogs(req *web.AdminUserRoleLogsReq) (*web.AdminUserRoleLogsResp, error) {
	limit, offset := req.PageSize, (req.Page-1)*req.PageSize
	logs, total, err := s.Ds.ListUserRoleLogs(offset, limit)
	if err != nil {
		logrus.Errorf("Ds.ListUserRoleLogs err: %s", err)
		return nil, web.ErrGetPostsFailed
	}
	ids := make([]int64, 0, len(logs)*2)
	for _, l := range logs {
		ids = append(ids, l.UserID, l.OperatorID)
	}
	usernames := usernamesOf(s.Ds, ids)
	items := make([]*web.AdminUserRoleLogItem, 0, len(logs))
	for _, l := range logs {
		items = append(items, &web.AdminUserRoleLogItem{
			ID:           l.ID,
			UserID:       l.UserID,
			Username:     usernames[l.UserID],
			OperatorID:   l.OperatorID,
			OperatorName: usernames[l.OperatorID],
			OldRoles:     l.OldRoles,
			NewRoles:     l.NewRoles,
			Action:       l.Action,
			CreatedOn:    l.CreatedOn,
		})
	}
	return (*web.AdminUserRoleLogsResp)(base.PageRespFrom(items, req.Page, req.PageSize, total)), nil
}

func newAdminSrv(s *base.DaoServant, wc core.WebCache, settings *sitesetting.Service) api.Admin {
	return &adminSrv{
		DaoServant:   s,
		wc:           wc,
		settings:     settings,
		serverUpTime: time.Now().Unix(),
	}
}
