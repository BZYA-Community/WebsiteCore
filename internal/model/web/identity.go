package web

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
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
	UserID   int64   `json:"user_id" binding:"required,min=1"`
	GroupIDs []int64 `json:"group_ids" binding:"max=100,dive,min=1"`
}

type IdentityLogsReq struct {
	BaseInfo `form:"-" binding:"-"`
	Page     int `form:"-" binding:"-"`
	PageSize int `form:"-" binding:"-"`
}

func (r *IdentityLogsReq) SetPageInfo(page, pageSize int) { r.Page, r.PageSize = page, pageSize }

type IdentityLogsResp base.PageResp
