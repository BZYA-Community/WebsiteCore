package web

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/model/joint"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/gin-gonic/gin"
)

type IdentityReq struct {
	BaseInfo `json:"-" binding:"-"`
}
type IdentityResp struct {
	Permissions []string `json:"permissions"`
}
type IdentityGroupsResp struct {
	Groups []*ms.IdentityGroup `json:"groups"`
}
type IdentityPermission struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}
type IdentityPermissionsResp struct {
	Permissions []IdentityPermission `json:"permissions"`
}

type SaveIdentityGroupReq struct {
	BaseInfo    `json:"-" binding:"-"`
	ID          int64    `json:"id" binding:"min=0"`
	Key         string   `json:"key" binding:"required,max=64"`
	Name        string   `json:"name" binding:"required,max=100"`
	Description string   `json:"description" binding:"max=500"`
	Permissions []string `json:"permissions" binding:"max=100"`
}

type DeleteIdentityGroupReq struct {
	BaseInfo `form:"-" binding:"-"`
	ID       int64 `form:"id" binding:"required,min=1"`
}

type SetUserIdentityReq struct {
	BaseInfo `json:"-" binding:"-"`
	UserID   int64   `json:"user_id" binding:"min=0"`
	UserIDs  []int64 `json:"user_ids" binding:"max=100,dive,min=1"`
	GroupIDs []int64 `json:"group_ids" binding:"max=100,dive,min=1"`
}

func (r *SetUserIdentityReq) Bind(c *gin.Context) error {
	if err := bindAny(c, r); err != nil {
		return err
	}
	if (r.UserID > 0) == (len(r.UserIDs) > 0) {
		return xerror.InvalidParams.WithDetails("请指定单个用户或 1–100 个批量用户")
	}
	return nil
}

type IdentityLogsReq struct {
	BaseInfo `form:"-" binding:"-"`
	Page     int `form:"-" binding:"-"`
	PageSize int `form:"-" binding:"-"`
}

func (r *IdentityLogsReq) SetPageInfo(page, pageSize int) { r.Page, r.PageSize = page, pageSize }

type IdentityLogsResp joint.PageResp
