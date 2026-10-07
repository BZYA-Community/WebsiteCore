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
	return gin.HandlersChain{chain.JWT(), chain.Authorize()}
}

func (s *adminSrv) ChangeUserStatus(req *web.ChangeUserStatusReq) error {
	return identityError(s.Ds.SetManagedUserStatus(req.User, req.ID, req.Status))
}

func (s *adminSrv) AdminUserDelete(req *web.AdminUserDeleteReq) error {
	return identityError(s.Ds.DeleteManagedUser(req.User, req.ID))
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
	if err := s.Ds.LoadUserIdentities(users...); err != nil {
		return nil, identityError(err)
	}
	items := make([]*web.AdminUserItem, 0, len(users))
	for _, user := range users {
		items = append(items, &web.AdminUserItem{
			ID:             user.ID,
			Nickname:       user.Nickname,
			Username:       user.Username,
			Phone:          maskPhone(user.Phone),
			Roles:          user.RoleList(),
			Identity:       user.DisplayIdentity(),
			Status:         user.Status,
			IsAdmin:        user.HasPermission("user.manage"),
			IsOperator:     user.IsOperator,
			IdentityGroups: user.GroupList(),
			Permissions:    user.PermissionList(),
			CreatedOn:      user.CreatedOn,
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
		Identity:       user.DisplayIdentity(),
		Status:         user.Status,
		IsAdmin:        user.HasPermission("user.manage"),
		IsOperator:     user.IsOperator,
		IdentityGroups: user.GroupList(),
		Permissions:    user.PermissionList(),
		CreatedOn:      user.CreatedOn,
	}, nil
}

func newAdminSrv(s *base.DaoServant, wc core.WebCache, settings *sitesetting.Service) api.Admin {
	return &adminSrv{
		DaoServant:   s,
		wc:           wc,
		settings:     settings,
		serverUpTime: time.Now().Unix(),
	}
}
