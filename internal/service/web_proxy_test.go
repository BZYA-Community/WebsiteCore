package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/gin-gonic/gin"
)

func TestWebClientIPHonorsOnlyConfiguredProxies(t *testing.T) {
	old := conf.WebServerSetting
	t.Cleanup(func() { conf.WebServerSetting = old })
	for _, tc := range []struct {
		name, remote, forwarded, real, want string
		trusted                             []string
	}{
		{name: "default ignores forged XFF", remote: "203.0.113.9:5000", forwarded: "198.51.100.25", want: "203.0.113.9"},
		{name: "default ignores forged real IP", remote: "203.0.113.9:5000", real: "198.51.100.25", want: "203.0.113.9"},
		{name: "untrusted peer ignores XFF", remote: "203.0.113.9:5000", forwarded: "198.51.100.25", trusted: []string{"127.0.0.1"}, want: "203.0.113.9"},
		{name: "explicit proxy forwards IP", remote: "127.0.0.1:5000", forwarded: "198.51.100.25", trusted: []string{"127.0.0.1"}, want: "198.51.100.25"},
		{name: "proxy chain stops at untrusted peer", remote: "192.0.2.10:5000", forwarded: "198.51.100.25, 203.0.113.9", trusted: []string{"192.0.2.0/24"}, want: "203.0.113.9"},
		{name: "IPv6 proxy", remote: "[::1]:5000", real: "2001:db8::10", trusted: []string{"::1"}, want: "2001:db8::10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conf.WebServerSetting = nil
			data, err := json.Marshal(map[string]any{"TrustedProxies": tc.trusted})
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &conf.WebServerSetting); err != nil {
				t.Fatal(err)
			}
			e := newWebEngine()
			e.GET("/client-ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })
			r := httptest.NewRequest(http.MethodGet, "/client-ip", nil)
			r.RemoteAddr = tc.remote
			r.Header.Set("X-Forwarded-For", tc.forwarded)
			r.Header.Set("X-Real-IP", tc.real)
			w := httptest.NewRecorder()
			e.ServeHTTP(w, r)
			if w.Code != http.StatusOK || w.Body.String() != tc.want {
				t.Fatalf("client IP = %q (status %d), want %q", w.Body.String(), w.Code, tc.want)
			}
		})
	}
}
