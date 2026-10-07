package web

import (
	"errors"

	api "github.com/BZYA-Community/WebsiteCore/auto/api/v1"
	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/model/joint"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/chain"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type identitySrv struct {
	api.UnimplementedIdentityServant
	*base.DaoServant
}

func (s *identitySrv) Chain() gin.HandlersChain { return gin.HandlersChain{chain.JwtLoose()} }
func (s *identitySrv) GetIdentity(req *web.IdentityReq) (*web.IdentityResp, error) {
	user := req.User
	if user == nil {
		user = &ms.User{Status: ms.UserStatusNormal}
		if err := s.Ds.LoadUserIdentities(user); err != nil {
			return nil, identityError(err)
		}
	}
	return &web.IdentityResp{Permissions: user.PermissionList()}, nil
}

func identityError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, authz.ErrDenied):
		return web.ErrNoPermission
	case errors.Is(err, authz.ErrInvalid):
		return xerror.InvalidParams
	case errors.Is(err, authz.ErrGroupInUse):
		return xerror.InvalidParams.WithDetails("身份组仍有成员，请先移除成员")
	case errors.Is(err, gorm.ErrRecordNotFound):
		return xerror.InvalidParams.WithDetails("用户或身份组不存在")
	default:
		logrus.WithError(err).Error("identity operation failed")
		return xerror.ServerError
	}
}

func (s *adminSrv) ListIdentityGroups() (*web.IdentityGroupsResp, error) {
	groups, err := s.Ds.ListIdentityGroups()
	if err != nil {
		return nil, identityError(err)
	}
	return &web.IdentityGroupsResp{Groups: groups}, nil
}
func (s *adminSrv) SaveIdentityGroup(req *web.SaveIdentityGroupReq) (*web.IdentityGroupsResp, error) {
	group, err := s.Ds.SaveIdentityGroup(req.User, &ms.IdentityGroup{ID: req.ID, Key: req.Key, Name: req.Name, Description: req.Description, Permissions: req.Permissions})
	if err != nil {
		return nil, identityError(err)
	}
	return &web.IdentityGroupsResp{Groups: []*ms.IdentityGroup{group}}, nil
}
func (s *adminSrv) DeleteIdentityGroup(req *web.DeleteIdentityGroupReq) error {
	return identityError(s.Ds.DeleteIdentityGroup(req.User, req.ID))
}
func (s *adminSrv) ListIdentityPermissions() (*web.IdentityPermissionsResp, error) {
	items := make([]web.IdentityPermission, 0)
	for _, key := range authz.All() {
		items = append(items, web.IdentityPermission{Key: key, Name: key})
	}
	return &web.IdentityPermissionsResp{Permissions: items}, nil
}
func (s *adminSrv) SetUserIdentityGroups(req *web.SetUserIdentityReq) error {
	if len(req.UserIDs) > 0 {
		return identityError(s.Ds.SetUsersIdentityGroups(req.User, req.UserIDs, req.GroupIDs))
	}
	return identityError(s.Ds.SetUserIdentityGroups(req.User, req.UserID, req.GroupIDs))
}
func (s *adminSrv) ListIdentityLogs(req *web.IdentityLogsReq) (*web.IdentityLogsResp, error) {
	rows, total, err := s.Ds.ListIdentityLogs((req.Page-1)*req.PageSize, req.PageSize)
	if err != nil {
		return nil, identityError(err)
	}
	return (*web.IdentityLogsResp)(joint.PageRespFrom(rows, req.Page, req.PageSize, total)), nil
}
