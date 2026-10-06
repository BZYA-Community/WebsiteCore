package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/transport/httpx"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/gin-gonic/gin"
)

type request struct {
	Name     string   `json:"name" form:"name" binding:"required"`
	User     *ms.User `json:"-" form:"-"`
	ID       int64    `json:"-" form:"-"`
	Page     int      `json:"-" form:"-"`
	PageSize int      `json:"-" form:"-"`
}

func (r *request) SetUser(user *ms.User)      { r.User = user }
func (r *request) SetUserId(id int64)         { r.ID = id }
func (r *request) SetPageInfo(page, size int) { r.Page, r.PageSize = page, size }

func TestBindHydratesOnlyAfterSuccessfulValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pagingConfig(t)
	for _, sentry := range []bool{false, true} {
		for _, body := range []string{`{"name":"Alice"}`, `{}`, `invalid json`} {
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequest("POST", "/example?page=-1&page_size=200", strings.NewReader(body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			user := &ms.User{Model: &ms.Model{ID: 7}, Username: "alice"}
			ctx.Set("USER", user)
			ctx.Set("UID", int64(7))
			input := &request{}
			err := httpx.New(sentry).Bind(ctx, input)
			if body == `{"name":"Alice"}` {
				if err != nil || input.Name != "Alice" || input.User != user || input.ID != 7 || input.Page != 1 || input.PageSize != 100 {
					t.Fatalf("bind sentry=%v: %+v, %v", sentry, input, err)
				}
			} else {
				if err == nil || input.User != nil || input.ID != 0 {
					t.Fatalf("invalid bind sentry=%v: %+v, %v", sentry, input, err)
				}
			}
		}
	}
}

func TestBindJsonRetainsFormCompatibilityAndGuestContext(t *testing.T) {
	pagingConfig(t)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/example?name=guest", nil)
	var input request
	if err := httpx.New(false).BindJson(ctx, &input); err != nil {
		t.Fatal(err)
	}
	if input.Name != "guest" || input.User != nil || input.ID != -1 {
		t.Fatalf("guest form = %+v", input)
	}
}

func TestRenderPreservesSuccessAndErrorEnvelopes(t *testing.T) {
	for _, failure := range []error{nil, xerror.InvalidParams} {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		httpx.New(false).Render(ctx, map[string]string{"value": "hello"}, failure)
		var body struct {
			Code int               `json:"code"`
			Msg  string            `json:"msg"`
			Data map[string]string `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if failure == nil {
			if rec.Code != http.StatusOK || body.Code != 0 || body.Msg != "success" || body.Data["value"] != "hello" {
				t.Fatalf("success = %d %+v", rec.Code, body)
			}
		} else {
			status, code := xerror.HttpStatusCode(failure)
			if rec.Code != status || body.Code != code || body.Msg != failure.Error() || body.Data != nil {
				t.Fatalf("error = %d %+v", rec.Code, body)
			}
		}
	}
}

func TestContextExtractionRejectsWrongTypes(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("UID", "7")
	ctx.Set("USER", "alice")
	ctx.Set("USERNAME", 7)
	if _, ok := httpx.UserIdFrom(ctx); ok {
		t.Fatal("string UID accepted")
	}
	if _, ok := httpx.UserFrom(ctx); ok {
		t.Fatal("string USER accepted")
	}
	if _, ok := httpx.UserNameFrom(ctx); ok {
		t.Fatal("numeric username accepted")
	}
}

// Initialize only the exported pagination config, without bootstrap, loggers,
// credentials, or backing services. JSON can populate its unexported type.
func pagingConfig(t *testing.T) {
	t.Helper()
	previous := conf.AppSetting
	conf.AppSetting = nil
	t.Cleanup(func() { conf.AppSetting = previous })
	if err := json.Unmarshal([]byte(`{"DefaultPageSize":20,"MaxPageSize":100}`), &conf.AppSetting); err != nil {
		t.Fatal(err)
	}
}
