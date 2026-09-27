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
	return newTestRateLimitEngineWithProxies(nil)
}

// newTestRateLimitEngineWithProxies 与生产 newWebEngine 一致地显式设置可信代理;
// 默认 nil = 严格模式(不信任任何代理头), X-Forwarded-For 不参与分桶键(#28评审🔴)
func newTestRateLimitEngineWithProxies(proxies []string) (*gin.Engine, *rateLimitStore) {
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
	if err := e.SetTrustedProxies(proxies); err != nil {
		panic(err)
	}
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

// TestRateLimitIgnoresSpoofedForwardedHeader 未配置可信代理时(严格模式),
// 客户端在同一连接上每请求换一个伪造的 X-Forwarded-For 无法获得新分桶键(#28评审🔴):
// 分桶只认 TCP 对端地址, 伪造转发头不能绕过限流
func TestRateLimitIgnoresSpoofedForwardedHeader(t *testing.T) {
	e, _ := newTestRateLimitEngine() // 与生产一致: SetTrustedProxies(nil)

	for i, spoofed := range []string{"8.8.8.8", "9.9.9.9"} {
		req := httptest.NewRequest("GET", "/v1/ok", nil)
		req.RemoteAddr = "4.4.4.4:12345"
		req.Header.Set("X-Forwarded-For", spoofed)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expect 200 got %d", i+1, w.Code)
		}
	}
	// 第3次再换一个伪造头 → 仍落在 4.4.4.4 的桶上 → 429(伪造不产生新桶)
	req := httptest.NewRequest("GET", "/v1/ok", nil)
	req.RemoteAddr = "4.4.4.4:12345"
	req.Header.Set("X-Forwarded-For", "99.99.99.99")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed XFF must not grant a new bucket, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestRateLimitHonorsXFFFromTrustedProxy 配置可信代理(反代部署)时, 代理追加的
// X-Forwarded-For 被采信, 真实客户端各自独立分桶 —— 修复不牺牲反代场景的功能性
func TestRateLimitHonorsXFFFromTrustedProxy(t *testing.T) {
	e, _ := newTestRateLimitEngineWithProxies([]string{"4.4.4.4"})

	forwarded := func(client string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/v1/ok", nil)
		req.RemoteAddr = "4.4.4.4:12345" // 反代的对端地址
		req.Header.Set("X-Forwarded-For", client)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		return w
	}
	// 代理转发的真实客户端 8.8.8.8 独立分桶
	for i := 0; i < 2; i++ {
		if w := forwarded("8.8.8.8"); w.Code != http.StatusOK {
			t.Fatalf("request %d: expect 200 got %d", i+1, w.Code)
		}
	}
	if w := forwarded("8.8.8.8"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("expect 429 for exhausted real client, got %d", w.Code)
	}
	// 同一代理下的另一真实客户端 9.9.9.9 拥有独立桶
	if w := forwarded("9.9.9.9"); w.Code != http.StatusOK {
		t.Fatalf("expect 200 for other real client, got %d", w.Code)
	}
	// 非代理来源(对端不在可信列表)伪造转发头 → 头被忽略, 每次换头仍落同一桶
	for i, spoofed := range []string{"9.9.9.9", "8.8.8.8"} {
		req := httptest.NewRequest("GET", "/v1/ok", nil)
		req.RemoteAddr = "6.6.6.6:12345"
		req.Header.Set("X-Forwarded-For", spoofed)
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("untrusted peer request %d: expect 200 got %d", i+1, w.Code)
		}
	}
	// 第3次: 头又换了, 但桶已随 6.6.6.6 耗尽 → 429(伪造头在非代理来源上无效)
	req := httptest.NewRequest("GET", "/v1/ok", nil)
	req.RemoteAddr = "6.6.6.6:12345"
	req.Header.Set("X-Forwarded-For", "7.7.7.7")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("untrusted peer's spoofed XFF must not separate buckets, got %d", w.Code)
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
