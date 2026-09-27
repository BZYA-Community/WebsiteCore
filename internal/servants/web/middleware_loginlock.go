// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/url"
	"strings"
	"sync"

	"github.com/BZYA-Community/WebsiteCore/internal/dao/cache"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/pkg/app"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/gin-gonic/gin"
)

const (
	_loginLockPath = "/v1/auth/login"
	// _maxLoginErrPerAccountIP 复合键(账号×IP)锁定阈值: 同一来源IP对同一账号
	// 1小时内失败达该次数即锁定(仅影响该IP对该账号), 正常用户与其余IP不受影响
	_maxLoginErrPerAccountIP = 10
	// _maxLoginLockAccountLen 账号归一化后长度上限, 超长视为非法账号不计数(防超长键)
	_maxLoginLockAccountLen = 64
	// 请求体解析与响应嗅探的字节上限(登录请求/响应均为小JSON)
	_maxLoginLockBodySize  = 64 << 10
	_maxLoginLockSniffSize = 8 << 10
)

// loginLockStore 计数存储(由 core.RedisCache 实现, 测试可注入内存实现)
type loginLockStore interface {
	GetCountLoginErrAccountIP(ctx context.Context, account string, ip string) (int64, error)
	DelCountLoginErrAccountIP(ctx context.Context, account string, ip string) error
	IncrCountLoginErrAccountIP(ctx context.Context, account string, ip string) error
	// 账号级影子计数(#28评审🟡): 与账号是否存在无关, 中间件按同一键与阈值统一判定
	GetCountLoginErrAccount(ctx context.Context, account string) (int64, error)
	DelCountLoginErrAccount(ctx context.Context, account string) error
	IncrCountLoginErrAccount(ctx context.Context, account string) error
}

// LoginLockout 登录失败锁定中间件(#28):
// 两级计数, 均与 pub.Login 的统一凭据错误(10003/10004)联动 —— 登录成功清零, 凭据失败递增:
//  1. 复合键(账号, 来源IP): 防止攻击者批量试错将任意账号锁死(单IP阈值 _maxLoginErrPerAccountIP);
//  2. 账号级影子计数(账号): 分布式撞库(多IP摊薄复合键)达到 _MaxAccountLoginErrTimes 时
//     统一返回锁定码 —— 该计数对"账号不存在"与"账号存在"使用同一键与阈值, 且在 handler
//     之前判定, 使两种账号在任意失败次数下响应完全一致, 消除枚举侧信道(#28评审🟡)。
//     pub.Login 内的用户ID计数保持不变作纵深防御(仅账号存在时可达, 行为与影子计数同步)。
func LoginLockout() gin.HandlerFunc {
	var (
		once  sync.Once
		store loginLockStore
	)
	return func(c *gin.Context) {
		if c.Request.URL.Path != _loginLockPath {
			c.Next()
			return
		}
		once.Do(func() { store = cache.NewRedisCache() })
		runLoginLock(store, c)
	}
}

// runLoginLock 中间件主体(测试注入store直接调用)
func runLoginLock(store loginLockStore, c *gin.Context) {
	ctx := c.Request.Context()

	// 1. 解析登录账号(读取后需原样恢复请求体)
	account, ok := lockoutAccount(c)
	if !ok {
		// 请求体非法时不参与锁定计数, 由 handler 走统一参数错误
		c.Next()
		return
	}
	// 账号归一化: 登录账号大小写不敏感, 键必须同步归一, 防止大小写变体绕过复合锁
	account = strings.ToLower(account)
	if len(account) > _maxLoginLockAccountLen {
		c.Next()
		return
	}
	ip := c.ClientIP()

	// 2. 复合键检查: 命中直接拒绝, 不再进入密码校验
	if n, err := store.GetCountLoginErrAccountIP(ctx, account, ip); err == nil && n >= _maxLoginErrPerAccountIP {
		c.Abort()
		app.NewResponse(c).ToErrorResponse(web.ErrTooManyLoginError)
		return
	}

	// 3. 账号级影子计数检查(#28评审🟡): 分布式撞库下(复合键被多IP摊薄)达到
	// 与 pub.Login 相同的阈值即返回同一锁定码。该检查不关心账号是否存在 ——
	// "账号不存在"与"账号存在"在每一次请求上都走同一条判定路径与同一个计数器,
	// 攻击者无法通过响应码(20014是否出现)区分两者, 填平原先的枚举侧信道。
	if n, err := store.GetCountLoginErrAccount(ctx, account); err == nil && n >= _MaxAccountLoginErrTimes {
		c.Abort()
		app.NewResponse(c).ToErrorResponse(web.ErrTooManyLoginError)
		return
	}

	// 4. 嗅探登录结果: 统一凭据错误(10003/10004)递增, 登录成功清零
	sw := &sniffWriter{ResponseWriter: c.Writer}
	c.Writer = sw
	c.Next()
	code, parsed := sniffRespCode(sw.buf.Bytes())
	switch {
	case !parsed:
	case code == 0:
		_ = store.DelCountLoginErrAccountIP(ctx, account, ip)
		_ = store.DelCountLoginErrAccount(ctx, account)
	case code == xerror.UnauthorizedAuthNotExist.StatusCode() || code == xerror.UnauthorizedAuthFailed.StatusCode():
		_ = store.IncrCountLoginErrAccountIP(ctx, account, ip)
		_ = store.IncrCountLoginErrAccount(ctx, account)
	}
}

// lockoutAccount 从登录请求体提取 username(JSON/form 均支持), 读取后恢复请求体
func lockoutAccount(c *gin.Context) (string, bool) {
	body, oversize, err := readRestoreBody(c)
	if err != nil || oversize || len(body) == 0 {
		return "", false
	}
	if strings.HasPrefix(c.ContentType(), "application/json") {
		var req struct {
			Username string `json:"username"`
		}
		if json.Unmarshal(body, &req) != nil {
			return "", false
		}
		return req.Username, req.Username != ""
	}
	vals, err := url.ParseQuery(string(body))
	if err != nil {
		return "", false
	}
	username := vals.Get("username")
	return username, username != ""
}

// readRestoreBody 读取至多 _maxLoginLockBodySize 字节并原样恢复请求体,
// 返回是否因超出上限而被截断(截断则跳过锁定计数)
func readRestoreBody(c *gin.Context) ([]byte, bool, error) {
	if c.Request.Body == nil {
		return nil, false, nil
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, _maxLoginLockBodySize))
	// 恢复: 已读前缀 + 底层剩余流
	c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), c.Request.Body))
	oversize := int64(len(body)) >= _maxLoginLockBodySize
	return body, oversize, err
}

// sniffWriter 捕获响应体以判定登录结果(仅中间件内部使用)
type sniffWriter struct {
	gin.ResponseWriter
	buf bytes.Buffer
}

func (w *sniffWriter) Write(b []byte) (int, error) {
	if w.buf.Len() < _maxLoginLockSniffSize {
		w.buf.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

// sniffRespCode 从 {"code": N} 响应体提取业务码
func sniffRespCode(b []byte) (int, bool) {
	if len(b) == 0 {
		return 0, false
	}
	var resp struct {
		Code int `json:"code"`
	}
	if json.Unmarshal(b, &resp) != nil {
		return 0, false
	}
	return resp.Code, true
}
