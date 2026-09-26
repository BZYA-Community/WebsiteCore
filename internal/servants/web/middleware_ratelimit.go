// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"sync"
	"time"

	"github.com/BZYA-Community/WebsiteCore/pkg/app"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

const (
	// 敏感路径清扫周期与空闲淘汰时长: 空闲桶被淘汰后重建, 控制内存占用
	_rateLimitSweepInterval = time.Minute
)

// _authRateLimitPaths 需要更严格限流的敏感路径(#28): 登录、注册、验证码获取/发送
var _authRateLimitPaths = map[string]struct{}{
	"/v1/auth/login":    {},
	"/v1/auth/register": {},
	"/v1/captcha":       {},
}

// rateLimitConfig 两级令牌桶参数(均为按客户端IP独立计)
type rateLimitConfig struct {
	// general* 通用API: 宽松上限, 覆盖浏览/发帖/上传等正常操作
	generalLimit rate.Limit
	generalBurst int
	// auth* 敏感路径(登录/注册/验证码): 严格上限
	authLimit rate.Limit
	authBurst int
	// idleTTL 空闲桶淘汰时间
	idleTTL time.Duration
}

func defaultRateLimitConfig() rateLimitConfig {
	return rateLimitConfig{
		generalLimit: rate.Limit(10),              // 持续 10 req/s (≈600 req/min) 每IP
		generalBurst: 200,                         // 突发 200, 页面资源并发加载不受影响
		authLimit:    rate.Every(6 * time.Second), // 持续 10 req/min 每IP
		authBurst:    10,                          // 突发 10
		idleTTL:      5 * time.Minute,
	}
}

type rateLimitEntry struct {
	general  *rate.Limiter
	auth     *rate.Limiter
	lastSeen time.Time
}

type rateLimitStore struct {
	cfg     rateLimitConfig
	mu      sync.Mutex
	entries map[string]*rateLimitEntry
}

func newRateLimitStore(cfg rateLimitConfig) *rateLimitStore {
	return &rateLimitStore{
		cfg:     cfg,
		entries: make(map[string]*rateLimitEntry),
	}
}

// allow 取令牌; sensitive 决定使用哪一级桶
func (s *rateLimitStore) allow(ip string, sensitive bool, now time.Time) bool {
	s.mu.Lock()
	e, ok := s.entries[ip]
	if !ok {
		e = &rateLimitEntry{
			general: rate.NewLimiter(s.cfg.generalLimit, s.cfg.generalBurst),
			auth:    rate.NewLimiter(s.cfg.authLimit, s.cfg.authBurst),
		}
		s.entries[ip] = e
	}
	e.lastSeen = now
	s.mu.Unlock()
	if sensitive {
		return e.auth.Allow()
	}
	return e.general.Allow()
}

// sweep 淘汰空闲桶, 返回清理后的桶数量(便于观测/测试)
func (s *rateLimitStore) sweep(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ip, e := range s.entries {
		if now.Sub(e.lastSeen) > s.cfg.idleTTL {
			delete(s.entries, ip)
		}
	}
	return len(s.entries)
}

func (s *rateLimitStore) sweepLoop() {
	t := time.NewTicker(_rateLimitSweepInterval)
	defer t.Stop()
	for now := range t.C {
		s.sweep(now)
	}
}

// RateLimit 全局限流中间件(#28):
// 按客户端IP分桶的两级令牌桶 —— 敏感路径(登录/注册/验证码)独立严格桶,
// 其余通用API宽松桶; 超限统一返回 xerror.TooManyRequests (code 10008 / HTTP 429)。
func RateLimit() gin.HandlerFunc {
	st := newRateLimitStore(defaultRateLimitConfig())
	go st.sweepLoop()
	return rateLimitWith(st)
}

// rateLimitWith 以指定store构造中间件(测试可注入小容量配置)
func rateLimitWith(st *rateLimitStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, sensitive := _authRateLimitPaths[c.Request.URL.Path]
		if !st.allow(c.ClientIP(), sensitive, time.Now()) {
			c.Abort()
			app.NewResponse(c).ToErrorResponse(xerror.TooManyRequests)
			return
		}
		c.Next()
	}
}
