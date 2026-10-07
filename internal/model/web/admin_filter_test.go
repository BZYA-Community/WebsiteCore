package web

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/gin-gonic/gin"
)

func TestAdminRegistrationRangeBinding(t *testing.T) {
	previous := conf.AppSetting
	conf.AppSetting = nil
	if err := json.Unmarshal([]byte(`{"DefaultPageSize":10,"MaxPageSize":100}`), &conf.AppSetting); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conf.AppSetting = previous })
	for _, query := range []struct {
		params string
		valid  bool
	}{
		{"", true}, {"registered_from=100&registered_to=200", true},
		{"registered_to=100", true}, {"registered_from=200&registered_to=100", false},
		{"registered_from=-1", false}, {"registered_to=invalid", false},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/?"+query.params, nil)
		c.Set("USER", &ms.User{Model: &ms.Model{ID: 7}})
		var req AdminUserListReq
		if err := req.Bind(c); (err == nil) != query.valid {
			t.Errorf("range %q: %v", query.params, err)
		}
		if query.valid && (req.User == nil || req.Page <= 0 || req.PageSize <= 0) {
			t.Errorf("range binding lost authentication or pagination: %+v", req)
		}
	}
}

func TestBatchIdentityBinding(t *testing.T) {
	for _, body := range []struct {
		json  string
		valid bool
	}{
		{`{"user_id":1,"group_ids":[]}`, true},
		{`{"user_ids":[1,2],"group_ids":[3]}`, true},
		{`{"user_ids":[1,2],"group_ids":[]}`, true},
		{`{"group_ids":[]}`, false},
		{`{"user_id":1,"user_ids":[2]}`, false},
		{`{"user_ids":[0]}`, false},
		{`{"user_ids":[1],"group_ids":[0]}`, false},
		{`{"user_ids":[` + strings.Repeat("1,", 100) + `1]}`, false},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/", strings.NewReader(body.json))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("USER", &ms.User{Model: &ms.Model{ID: 7}})
		var req SetUserIdentityReq
		if err := req.Bind(c); (err == nil) != body.valid {
			t.Errorf("batch %s: %v", body.json, err)
		}
		if body.valid && req.User == nil {
			t.Fatal("batch binding lost authenticated actor")
		}
	}
}
