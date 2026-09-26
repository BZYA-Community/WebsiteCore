// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package dao

import (
	"sync"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/sakila"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/search"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/slonik"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/storage"
	"github.com/alimy/tryst/cfg"
	"github.com/sirupsen/logrus"
)

var (
	ts     core.TweetSearchService
	ds     core.DataService
	oss    core.ObjectStorageService
	webDsa core.WebDataServantA

	_onceInitial sync.Once
)

func DataService() core.DataService {
	lazyInitial()
	return ds
}

func WebDataServantA() core.WebDataServantA {
	lazyInitial()
	return webDsa
}

func ObjectStorageService() core.ObjectStorageService {
	lazyInitial()
	return oss
}

func TweetSearchService() core.TweetSearchService {
	lazyInitial()
	return ts
}

func newAuthorizationManageService() (ams core.AuthorizationManageService) {
	if cfg.If("Gorm") {
		ams = jinzhu.NewAuthorizationManageService()
	} else if cfg.If("Sqlx") {
		ams = sakila.NewAuthorizationManageService()
	} else if cfg.If("Sqlc") && cfg.Any("Postgres", "PostgreSQL") {
		ams = slonik.NewAuthorizationManageService()
	} else {
		ams = jinzhu.NewAuthorizationManageService()
	}
	return
}

// lazyInitial do some package lazy initialize for performance
func lazyInitial() {
	_onceInitial.Do(func() {
		initDsX()
		initOSS()
		initTsX()
	})
}

func initDsX() {
	var dsVer, dsaVer core.VersionInfo
	if cfg.If("Gorm") {
		ds, dsVer = jinzhu.NewDataService()
		webDsa, dsaVer = jinzhu.NewWebDataServantA()
	} else if cfg.If("Sqlx") {
		ds, dsVer = sakila.NewDataService()
		webDsa, dsaVer = sakila.NewWebDataServantA()
	} else if cfg.If("Sqlc") && cfg.Any("Postgres", "PostgreSQL") {
		ds, dsVer = slonik.NewDataService()
		webDsa, dsaVer = slonik.NewWebDataServantA()
	} else {
		// default use gorm as orm for sql database
		ds, dsVer = jinzhu.NewDataService()
		webDsa, dsaVer = jinzhu.NewWebDataServantA()
	}
	logrus.Infof("use %s as core.DataService with version %s", dsVer.Name(), dsVer.Version())
	logrus.Infof("use %s as core.ServantA with version %s", dsaVer.Name(), dsaVer.Version())
}

func initOSS() {
	var v core.VersionInfo
	if cfg.If("AliOSS") {
		oss, v = storage.MustAliossService()
	} else if cfg.If("LocalOSS") {
		oss, v = storage.MustLocalossService()
	} else {
		// default use AliOSS as object storage service
		oss, v = storage.MustAliossService()
		logrus.Infof("use default AliOSS as object storage by version %s", v.Version())
		return
	}
	logrus.Infof("use %s as object storage by version %s", v.Name(), v.Version())
}

func initTsX() {
	var v core.VersionInfo
	ams := newAuthorizationManageService()
	cfg.On(cfg.Actions{
		"Zinc": func() {
			ts, v = search.NewZincTweetSearchService(ams)
		},
		"Meili": func() {
			ts, v = search.NewMeiliTweetSearchService(ams)
		},
	}, func() {
		ts, v = search.NewZincTweetSearchService(ams)
	})
	logrus.Infof("use %s as tweet search serice by version %s", v.Name(), v.Version())
	ts = search.NewBridgeTweetSearchService(ts)
}
