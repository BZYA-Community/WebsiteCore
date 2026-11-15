package web

import (
	"context"
	"mime"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	api "github.com/BZYA-Community/WebsiteCore/auto/api/v1"
	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/chain"
	"github.com/gin-gonic/gin"
)

type phoneExportTestData struct {
	core.DataService
	queried bool
}

func (d *phoneExportTestData) ListRegisteredUserPhones(context.Context) ([]string, error) {
	d.queried = true
	return []string{"01234567890"}, nil
}

type phoneExportTestAdmin struct {
	*adminSrv
	user *ms.User
}

func (s *phoneExportTestAdmin) Chain() gin.HandlersChain {
	return gin.HandlersChain{func(c *gin.Context) { c.Set("USER", s.user) }, chain.Authorize()}
}

func TestAdminUserPhoneExportDownload(t *testing.T) {
	for _, tc := range []struct {
		name     string
		operator bool
	}{
		{name: "delegated manager"},
		{name: "operator", operator: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := &phoneExportTestData{}
			router := gin.New()
			api.RegisterAdminServant(router, &phoneExportTestAdmin{
				adminSrv: &adminSrv{DaoServant: &base.DaoServant{BaseServant: base.NewBaseServant(), Ds: data}},
				user: &ms.User{Model: &ms.Model{ID: 1}, Status: ms.UserStatusNormal,
					IsOperator: tc.operator, Permissions: []string{authz.UserManage}},
			})
			res := httptest.NewRecorder()
			router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/v1/admin/user/phones/export", nil))
			if data.queried != tc.operator {
				t.Fatalf("queried phones = %v, want %v", data.queried, tc.operator)
			}
			disposition := res.Header().Get("Content-Disposition")
			if !tc.operator {
				if disposition != "" {
					t.Fatal("denied request returned a download")
				}
				return
			}
			media, params, err := mime.ParseMediaType(disposition)
			if err != nil || media != "attachment" ||
				!regexp.MustCompile(`^registered-user-phones-\d{8}-\d{6}\.xlsx$`).MatchString(params["filename"]) {
				t.Fatalf("unexpected Content-Disposition: %q", disposition)
			}
			if res.Code != http.StatusOK ||
				res.Header().Get("Content-Type") != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" ||
				res.Header().Get("Cache-Control") != "no-store" || res.Body.Len() == 0 {
				t.Fatalf("unexpected download response: status %d, headers %v", res.Code, res.Header())
			}
		})
	}
}
