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
)

type frontendWebService struct {
	*baseHttpService
}

func (s *frontendWebService) Name() string {
	return "FrontendWebService"
}

func (s *frontendWebService) Version() *semver.Version {
	return semver.MustParse("v0.1.0")
}

func (s *frontendWebService) OnInit() error {
	s.registerRoute(s, servants.RegisterFrontendWebServants)
	return nil
}

func (s *frontendWebService) String() string {
	return fmt.Sprintf("listen on %s\n", color.GreenString("http://%s:%s", conf.FrontendWebSetting.HttpIp, conf.FrontendWebSetting.HttpPort))
}

func newFrontendWebServiceService() Service {
	server := sharedHTTPServer(conf.FrontendWebSetting, newWebEngine)
	return &frontendWebService{
		baseHttpService: &baseHttpService{
			server: server,
		},
	}
}
