package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestJSONRoutingAndCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	e := newHTTPEngine(httpEngineOptions{API: true})
	e.GET("/item", func(c *gin.Context) { c.Status(http.StatusOK) })
	for _, tc := range []struct {
		method, path, message string
		status                int
	}{
		{"GET", "/missing", "Not Found", 404},
		{"POST", "/item", "Method Not Allowed", 405},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			var body struct {
				Code int    `json:"code"`
				Msg  string `json:"msg"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if rec.Code != tc.status || body.Code != tc.status || body.Msg != tc.message {
				t.Fatalf("response = %d %s", rec.Code, rec.Body.String())
			}
		})
	}
	req := httptest.NewRequest("GET", "/item", nil)
	req.Header.Set("Origin", "https://client.example.net")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("CORS response = %d %v", rec.Code, rec.Header())
	}
}

func TestDefaultEngineRouting(t *testing.T) {
	e := newHTTPEngine(httpEngineOptions{})
	e.GET("/object", func(c *gin.Context) { c.String(200, "object") })
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest("POST", "/object", nil))
	if rec.Code != 404 || rec.Body.String() != "404 page not found" {
		t.Fatalf("response = %d %s", rec.Code, rec.Body.String())
	}
}

type testHTTPSettings struct {
	addr        string
	read, write time.Duration
}

func (s testHTTPSettings) Addr() string                   { return s.addr }
func (s testHTTPSettings) GetReadTimeout() time.Duration  { return s.read }
func (s testHTTPSettings) GetWriteTimeout() time.Duration { return s.write }

func TestSharedHTTPAddressRetainsRoutesAndFirstPolicy(t *testing.T) {
	previous := httpServers
	httpServers = newServerPool[*httpServer]()
	t.Cleanup(func() { httpServers = previous })
	first := sharedHTTPServer(testHTTPSettings{"127.0.0.1:7777", time.Second, 2 * time.Second}, func() *gin.Engine { return newHTTPEngine(httpEngineOptions{API: true}) })
	first.e.GET("/first", func(c *gin.Context) { c.String(200, "first") })
	second := sharedHTTPServer(testHTTPSettings{"127.0.0.1:7777", 3 * time.Second, 4 * time.Second}, func() *gin.Engine { t.Fatal("shared address recreated"); return nil })
	second.e.GET("/second", func(c *gin.Context) { c.String(200, "second") })
	for _, path := range []string{"/first", "/second"} {
		rec := httptest.NewRecorder()
		first.server.Handler.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 {
			t.Fatalf("%s status = %d", path, rec.Code)
		}
	}
	if first.server.ReadTimeout != time.Second || first.server.WriteTimeout != 2*time.Second || first.server.MaxHeaderBytes != 1<<20 {
		t.Fatalf("first policy lost: %+v", first.server)
	}
}
