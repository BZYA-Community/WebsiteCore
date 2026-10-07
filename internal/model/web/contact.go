package web

import (
	"context"

	"github.com/gin-gonic/gin"
)

type SendContactCodeReq struct {
	BaseInfo `json:"-" binding:"-"`
	Mode     string          `json:"mode" binding:"required,oneof=email phone"`
	Address  string          `json:"address" binding:"required,max=254"`
	ClientIP string          `json:"-" binding:"-"`
	Context  context.Context `json:"-" binding:"-"`
}

func (r *SendContactCodeReq) Bind(c *gin.Context) error {
	r.ClientIP, r.Context = c.ClientIP(), c.Request.Context()
	return bindAny(c, r)
}

type VerifyContactCodeReq struct {
	BaseInfo `json:"-" binding:"-"`
	Mode     string          `json:"mode" binding:"required,oneof=email phone"`
	Address  string          `json:"address" binding:"required,max=254"`
	Code     string          `json:"code" binding:"required,len=6"`
	Context  context.Context `json:"-" binding:"-"`
}

func (r *VerifyContactCodeReq) Bind(c *gin.Context) error {
	r.Context = c.Request.Context()
	return bindAny(c, r)
}
