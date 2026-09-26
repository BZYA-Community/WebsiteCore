// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package service

import (
	"log"

	"github.com/BZYA-Community/WebsiteCore/pkg/types"
	"github.com/Masterminds/semver/v3"
	"github.com/alimy/tryst/cfg"
)

type Service interface {
	Name() string
	Version() *semver.Version
	OnInit() error
	OnStart() error
	OnStop() error
}

type baseService types.Empty

func (baseService) Name() string {
	return ""
}

func (baseService) Version() *semver.Version {
	return semver.MustParse("v0.0.1")
}

func (baseService) String() string {
	return ""
}

// MustInitService Initial service
func MustInitService() []Service {
	ss := newService()
	for _, s := range ss {
		if err := s.OnInit(); err != nil {
			log.Fatalf("initial %s service error: %s", s.Name(), err)
		}
	}
	return ss
}

func newService() (ss []Service) {
	// add all service if declared in features on config.yaml
	cfg.In(cfg.Actions{
		"Web": func() {
			ss = append(ss, newWebService())
		},
		"Frontend:Web": func() {
			ss = append(ss, newFrontendWebServiceService())
		},
		"Docs": func() {
			ss = append(ss, newDocsService())
		},
		"Pprof": func() {
			ss = append(ss, newPprofService())
		},
		"Metrics": func() {
			ss = append(ss, newMetricsService())
		},
	})
	return
}
