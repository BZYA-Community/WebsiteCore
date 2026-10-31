package web

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/model/joint"
	"github.com/gin-gonic/gin"
)

type ProjectListReq struct {
	BaseInfo `form:"-" binding:"-"`
	Keyword  string `form:"keyword"`
	Tag      string `form:"tag"`
	Followed bool   `form:"followed"`
	Page     int    `form:"-" binding:"-"`
	PageSize int    `form:"-" binding:"-"`
}

func (r *ProjectListReq) SetPageInfo(page, pageSize int) { r.Page, r.PageSize = page, pageSize }

type ProjectListResp joint.PageResp

type ProjectDetailReq struct {
	BaseInfo `form:"-" binding:"-"`
	ID       int64 `form:"id" binding:"required"`
}

type ProjectDetailResp struct {
	Project *ms.ProjectFormated `json:"project"`
}

type CreateProjectReq struct {
	BaseInfo `json:"-" binding:"-"`
	Title    string   `json:"title" binding:"required"`
	Intro    string   `json:"intro" binding:"required"`
	URL      string   `json:"url" binding:"required"`
	Images   []string `json:"images" binding:"required,min=1,max=6"`
	Tags     []string `json:"tags" binding:"max=8"`
	ClientIP string   `json:"-" binding:"-"`
}

type CreateProjectResp ms.ProjectFormated

func (r *CreateProjectReq) Bind(c *gin.Context) error {
	r.ClientIP = c.ClientIP()
	return bindAny(c, r)
}

type ProjectFollowReq struct {
	SimpleInfo `json:"-" binding:"-"`
	ID         int64 `json:"id" binding:"required"`
}

type ProjectFollowResp struct {
	Status      bool  `json:"status"`
	FollowCount int64 `json:"follow_count"`
}
