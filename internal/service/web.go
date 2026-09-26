// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package service

import (
	"fmt"
	"net/http"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/servants"
	web "github.com/BZYA-Community/WebsiteCore/internal/servants/web"
	"github.com/Masterminds/semver/v3"
	"github.com/fatih/color"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type webService struct {
	*baseHttpService
}

func (s *webService) Name() string {
	return "WebService"
}

func (s *webService) Version() *semver.Version {
	return semver.MustParse("v0.5.0")
}

func (s *webService) OnInit() error {
	s.registerRoute(s, servants.RegisterWebServants)
	return nil
}

func (s *webService) String() string {
	return fmt.Sprintf("listen on %s\n", color.GreenString("http://%s:%s", conf.WebServerSetting.HttpIp, conf.WebServerSetting.HttpPort))
}

func newWebEngine() *gin.Engine {
	e := gin.New()
	e.HandleMethodNotAllowed = true
	e.Use(gin.Logger())
	e.Use(gin.Recovery())

	// 跨域配置
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AddAllowHeaders("Authorization")
	e.Use(cors.New(corsConfig))
	// 使用Sentry hook
	if conf.UseSentryGin() {
		e.Use(sentrygin.New(sentrygin.Options{
			Repanic: true,
		}))
	}

	// 全局限流(#28): 按IP的两级令牌桶, 登录/注册/验证码等敏感路径更严格
	e.Use(web.RateLimit())
	// 登录失败锁定(#28): (账号×来源IP)复合键, 防止任意账号被远程锁死
	e.Use(web.LoginLockout())

	// 默认404
	e.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "Not Found",
		})
	})

	// 默认405
	e.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"code": 405,
			"msg":  "Method Not Allowed",
		})
	})

	return e
}

func newWebService() Service {
	addr := conf.WebServerSetting.HttpIp + ":" + conf.WebServerSetting.HttpPort
	server := httpServers.from(addr, func() *httpServer {
		engine := newWebEngine()
		return &httpServer{
			baseServer: newBaseServe(),
			e:          engine,
			server: &http.Server{
				Addr:           addr,
				Handler:        engine,
				ReadTimeout:    conf.WebServerSetting.GetReadTimeout(),
				WriteTimeout:   conf.WebServerSetting.GetWriteTimeout(),
				MaxHeaderBytes: 1 << 20,
			},
		}
	})
	return &webService{
		baseHttpService: &baseHttpService{
			server: server,
		},
	}
}
