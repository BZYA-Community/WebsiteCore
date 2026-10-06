package chain

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/gin-gonic/gin"
)

func TestRouteAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, method, path string
		user               *ms.User
		guest              []string
		loadErr            error
		want               bool
	}{
		{name: "guest public feed", method: "GET", path: "/v1/posts", guest: []string{authz.PostView}, want: true},
		{name: "guest permission removed", method: "GET", path: "/v1/posts"},
		{name: "guest cannot write even if misconfigured", method: "POST", path: "/v1/post", guest: []string{authz.PostCreate}},
		{name: "database failure denies guest", method: "GET", path: "/v1/posts", loadErr: errors.New("unavailable")},
		{name: "legacy admin flag cannot grant access", method: "GET", path: "/v1/admin/user/list", user: &ms.User{Model: &ms.Model{ID: 1}, Status: ms.UserStatusNormal, IsAdmin: true}},
		{name: "explicit permission", method: "GET", path: "/v1/admin/user/list", user: &ms.User{Model: &ms.Model{ID: 1}, Status: ms.UserStatusNormal, Permissions: []string{authz.UserManage}}, want: true},
		{name: "operator", method: "GET", path: "/v1/admin/user/list", user: &ms.User{Model: &ms.Model{ID: 1}, Status: ms.UserStatusNormal, IsOperator: true}, want: true},
		{name: "banned operator", method: "GET", path: "/v1/admin/user/list", user: &ms.User{Model: &ms.Model{ID: 1}, Status: ms.UserStatusClosed, IsOperator: true}},
		{name: "unknown route denies operator", method: "POST", path: "/v1/forgotten", user: &ms.User{Model: &ms.Model{ID: 1}, Status: ms.UserStatusNormal, IsOperator: true}},
		{name: "course comment needs view", method: "POST", path: "/v1/course/comment", user: &ms.User{Model: &ms.Model{ID: 1}, Status: ms.UserStatusNormal, Permissions: []string{authz.CommentCreate}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			reached := false
			router.Handle(tc.method, tc.path, func(c *gin.Context) {
				if tc.user != nil {
					c.Set("USER", tc.user)
				}
			}, authorizeWith(func(users ...*ms.User) error { users[0].Permissions = tc.guest; return tc.loadErr }), func(c *gin.Context) { reached = true; c.Status(204) })
			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(tc.method, tc.path, nil))
			if reached != tc.want {
				t.Fatalf("handler reached=%v, want %v", reached, tc.want)
			}
		})
	}
}

// Keep middleware policy in sync with the source of truth, not generated code.
func TestEveryProtectedRouteHasPolicy(t *testing.T) {
	files, err := filepath.Glob("../../../mirc/web/v1/*.go")
	if err != nil || len(files) == 0 {
		t.Fatal("API definitions missing", err)
	}
	found := map[string]bool{}
	for _, path := range files {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			spec, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			st, ok := spec.Type.(*ast.StructType)
			if !ok {
				return false
			}
			protected := false
			for _, field := range st.Fields.List {
				if field.Tag != nil {
					v, _ := strconv.Unquote(field.Tag.Value)
					if reflect.StructTag(v).Get("mir") == "v1,chain" {
						protected = true
					}
				}
			}
			if !protected || spec.Name.Name == "Identity" {
				return false
			}
			for _, field := range st.Fields.List {
				fn, ok := field.Type.(*ast.FuncType)
				if !ok || field.Tag == nil {
					continue
				}
				verb, ok := fn.Params.List[0].Type.(*ast.Ident)
				if !ok {
					t.Fatal("unsupported route method")
				}
				tag, _ := strconv.Unquote(field.Tag.Value)
				key := strings.ToUpper(verb.Name) + " /v1/" + strings.TrimPrefix(reflect.StructTag(tag).Get("mir"), "/")
				found[key] = true
				if _, ok := routeRules[key]; !ok {
					t.Errorf("route has no policy: %s", key)
				}
			}
			return false
		})
	}
	for route, rule := range routeRules {
		if !found[route] {
			t.Errorf("policy has no route: %s", route)
		}
		for _, p := range append(append([]string{}, rule.All...), rule.Any...) {
			if !authz.Valid(p) {
				t.Errorf("unknown permission %s", p)
			}
		}
	}
}
