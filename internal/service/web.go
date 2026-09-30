// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package service

import (
	"fmt"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/servants"
	"github.com/Masterminds/semver/v3"
	"github.com/fatih/color"
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
	return newHTTPEngine(httpEngineOptions{API: true, Sentry: conf.UseSentryGin()})
}

func newWebService() Service {
	server := sharedHTTPServer(conf.WebServerSetting, newWebEngine)
	return &webService{
		baseHttpService: &baseHttpService{
			server: server,
		},
	}
}
