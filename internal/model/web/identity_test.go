package web

import (
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/gin-gonic/gin"
)

func TestUserInfoBindingRetainsAuthenticatedIdentity(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	user := &ms.User{Model: &ms.Model{ID: 7}, Username: "fixture", Status: ms.UserStatusNormal}
	c.Set("USER", user)
	c.Set("USERNAME", user.Username)
	var req UserInfoReq
	if err := req.Bind(c); err != nil || req.User != user || req.Username != user.Username {
		t.Fatalf("authenticated identity was not bound: %+v, %v", req, err)
	}
	c, _ = gin.CreateTestContext(nil)
	if err := (&UserInfoReq{}).Bind(c); err == nil {
		t.Fatal("accepted anonymous account-info request")
	}
}
