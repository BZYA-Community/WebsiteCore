// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

// fakeLoginLockStore 内存版计数(无需Redis/配置文件): 复合键与账号级影子计数共用一张表
type fakeLoginLockStore struct {
	mu     sync.Mutex
	counts map[string]int64
}

func newFakeLoginLockStore() *fakeLoginLockStore {
	return &fakeLoginLockStore{counts: make(map[string]int64)}
}

func lockKey(account, ip string) string {
	return account + "|" + ip
}

// acctLockKey 账号级影子计数键(与复合键共表, 前缀隔离)
func acctLockKey(account string) string {
	return "acct|" + account
}

func (f *fakeLoginLockStore) GetCountLoginErrAccountIP(_ context.Context, account string, ip string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[lockKey(account, ip)], nil
}

func (f *fakeLoginLockStore) DelCountLoginErrAccountIP(_ context.Context, account string, ip string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.counts, lockKey(account, ip))
	return nil
}

func (f *fakeLoginLockStore) IncrCountLoginErrAccountIP(_ context.Context, account string, ip string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counts[lockKey(account, ip)]++
	return nil
}

func (f *fakeLoginLockStore) GetCountLoginErrAccount(_ context.Context, account string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[acctLockKey(account)], nil
}

func (f *fakeLoginLockStore) DelCountLoginErrAccount(_ context.Context, account string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.counts, acctLockKey(account))
	return nil
}

func (f *fakeLoginLockStore) IncrCountLoginErrAccount(_ context.Context, account string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.counts[acctLockKey(account)]++
	return nil
}

// lockTestEnv 登录路由测试环境: 伪handler按 respCode 返回与 mir Render 一致的JSON
type lockTestEnv struct {
	store        *fakeLoginLockStore
	handlerCalls int
	respCode     int
	gotUsername  string
	gotBody      string
}

func newLockTestEnv() (*gin.Engine, *lockTestEnv) {
	gin.SetMode(gin.TestMode)
	env := &lockTestEnv{store: newFakeLoginLockStore(), respCode: 10004}
	e := gin.New()
	// 与生产 newWebEngine 一致: 严格模式, 分桶键取 TCP 对端地址(#28评审🔴)
	if err := e.SetTrustedProxies(nil); err != nil {
		panic(err)
	}
	e.Use(func(c *gin.Context) { runLoginLock(env.store, c) })
	e.POST("/v1/auth/login", func(c *gin.Context) {
		env.handlerCalls++
		body, _ := io.ReadAll(c.Request.Body)
		env.gotBody = string(body)
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if c.ContentType() == "application/json" {
			_ = json.Unmarshal(body, &req)
		} else if vals, err := url.ParseQuery(string(body)); err == nil {
			req.Username = vals.Get("username")
		}
		env.gotUsername = req.Username
		if env.respCode == 0 {
			c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"code": env.respCode, "msg": "error"})
	})
	e.POST("/v1/other", func(c *gin.Context) {
		env.handlerCalls++
		c.JSON(http.StatusOK, gin.H{"code": 0})
	})
	return e, env
}

func postLogin(e *gin.Engine, ip, username string) *httptest.ResponseRecorder {
	return postLoginBody(e, ip, "application/json",
		fmt.Sprintf(`{"username":%q,"password":"wrong-pass"}`, username))
}

func postLoginBody(e *gin.Engine, ip, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(body))
	req.RemoteAddr = ip + ":12345"
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	return w
}

// TestLoginLockoutCompositeKey 同一(IP,账号)10次失败后第11次直接拒绝, 不再进入handler
func TestLoginLockoutCompositeKey(t *testing.T) {
	e, env := newLockTestEnv()

	for i := 0; i < _maxLoginErrPerAccountIP; i++ {
		w := postLogin(e, "10.0.0.1", "victim")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expect 401 got %d body=%s", i+1, w.Code, w.Body.String())
		}
	}
	if env.handlerCalls != _maxLoginErrPerAccountIP {
		t.Fatalf("expect %d handler calls, got %d", _maxLoginErrPerAccountIP, env.handlerCalls)
	}

	// 第11次: 复合键命中 → 20014, handler不再被调用
	w := postLogin(e, "10.0.0.1", "victim")
	if !strings.Contains(w.Body.String(), `"code":20014`) {
		t.Fatalf("expect code 20014, got %s", w.Body.String())
	}
	if env.handlerCalls != _maxLoginErrPerAccountIP {
		t.Fatalf("locked request must not reach handler, calls=%d", env.handlerCalls)
	}
}

// TestLoginLockoutDoesNotAffectOtherIP 其余来源IP对同一账号不受影响(不可再定向锁死任意账号)
func TestLoginLockoutDoesNotAffectOtherIP(t *testing.T) {
	e, env := newLockTestEnv()

	for i := 0; i <= _maxLoginErrPerAccountIP; i++ {
		postLogin(e, "10.0.0.1", "victim")
	}
	callsBefore := env.handlerCalls
	w := postLogin(e, "10.0.0.2", "victim")
	if w.Code != http.StatusUnauthorized || env.handlerCalls != callsBefore+1 {
		t.Fatalf("other ip should pass through: status=%d calls=%d want=%d", w.Code, env.handlerCalls, callsBefore+1)
	}
}

// TestLoginLockoutAccountNormalization 账号大小写归一, 变体无法绕过复合锁
func TestLoginLockoutAccountNormalization(t *testing.T) {
	e, env := newLockTestEnv()

	for i := 0; i < _maxLoginErrPerAccountIP; i++ {
		postLogin(e, "10.0.0.3", "victim")
	}
	w := postLogin(e, "10.0.0.3", "ViCtIm")
	if !strings.Contains(w.Body.String(), `"code":20014`) {
		t.Fatalf("case variant should be locked too, got %s", w.Body.String())
	}
	if env.handlerCalls != _maxLoginErrPerAccountIP {
		t.Fatalf("case variant must not reach handler, calls=%d", env.handlerCalls)
	}
}

// TestLoginLockoutUniformForUnknownAccount 账号不存在与密码错误走完全相同的锁定路径(#28)
// —— 这里锁定行为对任意账号名一致(伪handler恒返回统一凭据错误10004)
func TestLoginLockoutUniformForUnknownAccount(t *testing.T) {
	e, env := newLockTestEnv()

	for i := 0; i < _maxLoginErrPerAccountIP; i++ {
		postLogin(e, "10.0.0.4", "ghost")
	}
	w := postLogin(e, "10.0.0.4", "ghost")
	if !strings.Contains(w.Body.String(), `"code":20014`) {
		t.Fatalf("unknown account must lock with identical code, got %s", w.Body.String())
	}
	if env.handlerCalls != _maxLoginErrPerAccountIP {
		t.Fatalf("expect %d calls, got %d", _maxLoginErrPerAccountIP, env.handlerCalls)
	}
}

// TestLoginLockoutSuccessResets 登录成功清零复合键计数
func TestLoginLockoutSuccessResets(t *testing.T) {
	e, env := newLockTestEnv()

	for i := 0; i < _maxLoginErrPerAccountIP-1; i++ {
		postLogin(e, "10.0.0.5", "victim")
	}
	// 成功登录
	env.respCode = 0
	w := postLogin(e, "10.0.0.5", "victim")
	if w.Code != http.StatusOK {
		t.Fatalf("expect 200 got %d", w.Code)
	}
	if n, _ := env.store.GetCountLoginErrAccountIP(context.Background(), "victim", "10.0.0.5"); n != 0 {
		t.Fatalf("counter should reset after success, got %d", n)
	}
	// 清零后再失败9次仍可进入handler(未达阈值)
	env.respCode = 10004
	callsBefore := env.handlerCalls
	for i := 0; i < _maxLoginErrPerAccountIP-1; i++ {
		postLogin(e, "10.0.0.5", "victim")
	}
	if env.handlerCalls != callsBefore+_maxLoginErrPerAccountIP-1 {
		t.Fatalf("counter was not reset, calls=%d", env.handlerCalls)
	}
}

// TestLoginLockoutAccountLevelDistributedDodge 分布式撞库: 失败次数被摊薄到多个IP,
// 各复合键均未达阈值, 但账号级影子计数达到 _MaxAccountLoginErrTimes 后直接返回20014,
// 且该判定与账号是否存在无关(#28评审🟡) —— 伪handler恒返回10004即"账号不存在"路径,
// 与 pub.Login 对真实账号的20014走同一中间件判定、同一阈值、同一响应码
func TestLoginLockoutAccountLevelDistributedDodge(t *testing.T) {
	e, env := newLockTestEnv()

	// 5个来源IP轮转, 每IP失败9次(复合键9<10), 全部进入handler → 影子计数45
	for n := 0; n < _MaxAccountLoginErrTimes-5; n++ {
		ip := fmt.Sprintf("10.1.0.%d", n%5+1)
		w := postLogin(e, ip, "victim")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expect 401 got %d", n+1, w.Code)
		}
	}
	if env.handlerCalls != _MaxAccountLoginErrTimes-5 {
		t.Fatalf("expect %d handler calls, got %d", _MaxAccountLoginErrTimes-5, env.handlerCalls)
	}
	// 再失败5次(每IP第10次, 复合键预检9<10仍放行) → 影子计数恰好达到阈值
	for n := 0; n < 5; n++ {
		ip := fmt.Sprintf("10.1.0.%d", n%5+1)
		postLogin(e, ip, "victim")
	}
	if n, _ := env.store.GetCountLoginErrAccount(context.Background(), "victim"); n != _MaxAccountLoginErrTimes {
		t.Fatalf("shadow counter expect %d got %d", _MaxAccountLoginErrTimes, n)
	}
	if env.handlerCalls != _MaxAccountLoginErrTimes {
		t.Fatalf("expect %d handler calls, got %d", _MaxAccountLoginErrTimes, env.handlerCalls)
	}

	// 第51次用全新IP(复合键0, 不会被复合锁抢先): 影子计数命中 → 20014, 不进handler
	w := postLogin(e, "10.1.0.99", "victim")
	if !strings.Contains(w.Body.String(), `"code":20014`) {
		t.Fatalf("expect code 20014 from account-level shadow counter, got %s", w.Body.String())
	}
	if env.handlerCalls != _MaxAccountLoginErrTimes {
		t.Fatalf("locked request must not reach handler, calls=%d", env.handlerCalls)
	}
}

// TestLoginLockoutAccountLevelResetOnSuccess 登录成功同时清零复合键与账号级影子计数
func TestLoginLockoutAccountLevelResetOnSuccess(t *testing.T) {
	e, env := newLockTestEnv()

	for i := 0; i < 5; i++ {
		postLogin(e, "10.2.0.1", "victim")
	}
	if n, _ := env.store.GetCountLoginErrAccount(context.Background(), "victim"); n != 5 {
		t.Fatalf("shadow counter expect 5 got %d", n)
	}
	env.respCode = 0
	if w := postLogin(e, "10.2.0.1", "victim"); w.Code != http.StatusOK {
		t.Fatalf("expect 200 got %d", w.Code)
	}
	if n, _ := env.store.GetCountLoginErrAccount(context.Background(), "victim"); n != 0 {
		t.Fatalf("shadow counter should reset after success, got %d", n)
	}
	if n, _ := env.store.GetCountLoginErrAccountIP(context.Background(), "victim", "10.2.0.1"); n != 0 {
		t.Fatalf("composite counter should reset after success, got %d", n)
	}
}

// TestLoginLockoutFormBody 支持表单编码的登录请求
func TestLoginLockoutFormBody(t *testing.T) {
	e, env := newLockTestEnv()

	for i := 0; i < _maxLoginErrPerAccountIP; i++ {
		w := postLoginBody(e, "10.0.0.6", "application/x-www-form-urlencoded",
			url.Values{"username": {"victim"}, "password": {"wrong"}}.Encode())
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expect 401 got %d", i+1, w.Code)
		}
	}
	if env.gotUsername != "victim" {
		t.Fatalf("body must be restored intact, handler got username=%q", env.gotUsername)
	}
	w := postLoginBody(e, "10.0.0.6", "application/x-www-form-urlencoded",
		url.Values{"username": {"victim"}, "password": {"wrong"}}.Encode())
	if !strings.Contains(w.Body.String(), `"code":20014`) {
		t.Fatalf("expect lockout for form login, got %s", w.Body.String())
	}
}

// TestLoginLockoutBodyRestored 超限请求体跳过计数但完整透传
func TestLoginLockoutBodyRestored(t *testing.T) {
	e, env := newLockTestEnv()

	big := `{"username":"victim","password":"` + strings.Repeat("x", _maxLoginLockBodySize) + `"}`
	w := postLoginBody(e, "10.0.0.7", "application/json", big)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expect handler reached, got %d", w.Code)
	}
	if len(env.gotBody) != len(big) {
		t.Fatalf("body must be restored fully: got %d bytes, want %d", len(env.gotBody), len(big))
	}
	// 超限请求不计入锁定
	if n, _ := env.store.GetCountLoginErrAccountIP(context.Background(), "victim", "10.0.0.7"); n != 0 {
		t.Fatalf("oversize body must not be counted, got %d", n)
	}
}

// TestLoginLockoutPassThroughNonLoginPath 非登录路径不参与锁定逻辑
func TestLoginLockoutPassThroughNonLoginPath(t *testing.T) {
	e, env := newLockTestEnv()

	req := httptest.NewRequest("POST", "/v1/other", strings.NewReader(`{"anything":1}`))
	req.RemoteAddr = "10.0.0.8:12345"
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	if w.Code != http.StatusOK || env.handlerCalls != 1 {
		t.Fatalf("non-login path must pass through: status=%d calls=%d", w.Code, env.handlerCalls)
	}
}

func TestSniffRespCode(t *testing.T) {
	cases := []struct {
		in     string
		code   int
		parsed bool
	}{
		{`{"code":10004,"msg":"错误码: 10004"}`, 10004, true},
		{`{"code":0,"msg":"success","data":{}}`, 0, true},
		{``, 0, false},
		{`not-json`, 0, false},
	}
	for _, c := range cases {
		code, parsed := sniffRespCode([]byte(c.in))
		if code != c.code || parsed != c.parsed {
			t.Fatalf("sniffRespCode(%q) = (%d,%v), want (%d,%v)", c.in, code, parsed, c.code, c.parsed)
		}
	}
}
