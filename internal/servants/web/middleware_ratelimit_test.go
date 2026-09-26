// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// newTestRateLimitEngine 构造小容量限流测试环境: 通用/敏感桶各突发2
func newTestRateLimitEngine() (*gin.Engine, *rateLimitStore) {
	gin.SetMode(gin.TestMode)
	cfg := rateLimitConfig{
		generalLimit: rate.Every(time.Minute),
		generalBurst: 2,
		authLimit:    rate.Every(time.Minute),
		authBurst:    2,
		idleTTL:      5 * time.Minute,
	}
	st := newRateLimitStore(cfg)
	e := gin.New()
	e.Use(rateLimitWith(st))
	e.GET("/v1/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0})
	})
	e.POST("/v1/auth/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"code": 0})
	})
	return e, st
}

func doRequest(e *gin.Engine, method, path, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = ip + ":12345"
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	return w
}

func TestRateLimitGeneralBucket(t *testing.T) {
	e, _ := newTestRateLimitEngine()

	// 突发额度内放行
	for i := 0; i < 2; i++ {
		w := doRequest(e, "GET", "/v1/ok", "1.1.1.1")
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expect 200 got %d", i+1, w.Code)
		}
	}
	// 第3个请求超限 → 429 + xerror.TooManyRequests
	w := doRequest(e, "GET", "/v1/ok", "1.1.1.1")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expect 429 got %d, body=%s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); !strings.Contains(body, `"code":10008`) {
		t.Fatalf("expect code 10008 in body, got %s", body)
	}
}

func TestRateLimitPerIPIndependent(t *testing.T) {
	e, _ := newTestRateLimitEngine()

	// IP 1.1.1.1 耗尽突发额度
	doRequest(e, "GET", "/v1/ok", "1.1.1.1")
	doRequest(e, "GET", "/v1/ok", "1.1.1.1")
	if w := doRequest(e, "GET", "/v1/ok", "1.1.1.1"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("expect 429 for exhausted ip, got %d", w.Code)
	}
	// 不同IP拥有独立桶, 不受影响
	w := doRequest(e, "GET", "/v1/ok", "2.2.2.2")
	if w.Code != http.StatusOK {
		t.Fatalf("ip 2.2.2.2 expect 200 got %d body=%s", w.Code, w.Body.String())
	}
}

func TestRateLimitAuthPathStricterAndIndependent(t *testing.T) {
	e, _ := newTestRateLimitEngine()
	ip := "3.3.3.3"

	// 敏感路径使用独立桶: 通用桶额度耗尽不影响敏感桶
	for i := 0; i < 2; i++ {
		w := doRequest(e, "POST", "/v1/auth/login", ip)
		if w.Code != http.StatusOK {
			t.Fatalf("auth request %d: expect 200 got %d", i+1, w.Code)
		}
	}
	w := doRequest(e, "POST", "/v1/auth/login", ip)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expect auth bucket 429 got %d body=%s", w.Code, w.Body.String())
	}
	// 通用路径此时仍可用(通用桶未耗尽)
	w = doRequest(e, "GET", "/v1/ok", ip)
	if w.Code != http.StatusOK {
		t.Fatalf("general path should pass, got %d", w.Code)
	}
}

func TestRateLimitClientIPFromForwardedHeader(t *testing.T) {
	e, _ := newTestRateLimitEngine()

	// gin 默认信任代理头, X-Forwarded-For 决定分桶键
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/v1/ok", nil)
		req.RemoteAddr = "4.4.4.4:12345"
		req.Header.Set("X-Forwarded-For", "8.8.8.8")
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("forwarded request %d: expect 200 got %d", i+1, w.Code)
		}
	}
	// 同一转发IP第3次 → 429(桶按 XFF 键)
	req := httptest.NewRequest("GET", "/v1/ok", nil)
	req.RemoteAddr = "4.4.4.4:12345"
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expect 429 for exhausted forwarded ip, got %d", w.Code)
	}
	// 同socket不同转发IP → 新桶仍放行
	req = httptest.NewRequest("GET", "/v1/ok", nil)
	req.RemoteAddr = "4.4.4.4:12345"
	req.Header.Set("X-Forwarded-For", "9.9.9.9")
	w = httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expect 200 for new forwarded ip, got %d", w.Code)
	}
}

func TestRateLimitSweepEvictsIdleEntries(t *testing.T) {
	e, st := newTestRateLimitEngine()

	doRequest(e, "GET", "/v1/ok", "5.5.5.5")
	if got := st.sweep(time.Now()); got != 1 {
		t.Fatalf("expect 1 live entry, got %d", got)
	}
	// 超过 idleTTL 后清扫
	if got := st.sweep(time.Now().Add(st.cfg.idleTTL + time.Second)); got != 0 {
		t.Fatalf("expect idle entry evicted, got %d entries", got)
	}
}
