// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"context"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/model/joint"

	api "github.com/BZYA-Community/WebsiteCore/auto/api/v1"
	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/chain"
	"github.com/BZYA-Community/WebsiteCore/internal/sitesetting"
	"github.com/gin-gonic/gin"
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
	if req.User == nil {
		return web.ErrNoPermission
	}
	if err := s.Ds.ChangeAccountStatus(req.User.ID, req.ID, req.Status); err != nil {
		return accessError(err)
	}
	s.expireManagedUser(req.ID)
	return nil
}

func (s *adminSrv) AdminUserDelete(req *web.AdminUserDeleteReq) error {
	if req.User == nil {
		return web.ErrNoPermission
	}
	user, err := s.Ds.GetUserByID(req.ID)
	if err != nil {
		return web.ErrNoExistUsername
	}
	if err := s.Ds.DeleteMember(req.User.ID, req.ID); err != nil {
		return accessError(err)
	}
	onChangeUsernameEvent(req.ID, user.Username)
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
	return dbr.MaskPhone(phone)
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
			ID:             user.ID,
			Nickname:       user.Nickname,
			Username:       user.Username,
			Phone:          maskPhone(user.Phone),
			Roles:          user.RoleList(),
			AccountType:    user.AccountType,
			MemberIdentity: user.MemberIdentity,
			IsMentor:       user.IsMentor,
			Status:         user.Status,

			CreatedOn: user.CreatedOn,
		})
	}
	return (*web.AdminUserListResp)(joint.PageRespFrom(items, req.Page, req.PageSize, total)), nil
}

// AdminUserDetail 用户管理·用户详情(完整手机号)
func (s *adminSrv) AdminUserDetail(req *web.AdminUserDetailReq) (*web.AdminUserDetailResp, error) {
	user, err := s.Ds.GetUserByID(req.ID)
	if err != nil || user.Model == nil || user.ID <= 0 {
		return nil, web.ErrNoExistUsername
	}
	return &web.AdminUserDetailResp{
		ID:             user.ID,
		Nickname:       user.Nickname,
		Username:       user.Username,
		Phone:          user.Phone,
		Roles:          user.RoleList(),
		AccountType:    user.AccountType,
		MemberIdentity: user.MemberIdentity,
		IsMentor:       user.IsMentor,
		Status:         user.Status,

		CreatedOn: user.CreatedOn,
	}, nil
}

// AdminUserRoleChange 用户管理·变更用户角色
// 权限规则: 运维可管理所有角色; 管理员只能管理 mentor/auditor/admin 且不可变更运维账号
func (s *adminSrv) AdminUserRoleChange(req *web.AdminUserRoleReq) error {
	if req.User == nil {
		return web.ErrNoPermission
	}
	// Auditor is set with member access. The only standalone role removal is
	// an Operator disabling a dedicated Admin; arbitrary grants are forbidden.
	if req.Role != ms.RoleAdmin || req.Action != "remove" {
		return web.ErrRoleChangeNoPermission
	}
	if err := s.Ds.RemoveAdminRole(req.User.ID, req.UserID); err != nil {
		return accessError(err)
	}
	s.expireManagedUser(req.UserID)
	return nil
}

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
	return (*web.AdminUserRoleLogsResp)(joint.PageRespFrom(items, req.Page, req.PageSize, total)), nil
}

func newAdminSrv(s *base.DaoServant, wc core.WebCache, settings *sitesetting.Service) api.Admin {
	return &adminSrv{
		DaoServant:   s,
		wc:           wc,
		settings:     settings,
		serverUpTime: time.Now().Unix(),
	}
}
