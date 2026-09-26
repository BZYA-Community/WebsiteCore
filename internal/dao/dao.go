// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package dao

import (
	"sync"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/search"
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

func newAuthorizationManageService() core.AuthorizationManageService {
	return jinzhu.NewAuthorizationManageService()
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
	ds, dsVer = jinzhu.NewDataService()
	webDsa, dsaVer = jinzhu.NewWebDataServantA()
	logrus.Infof("use %s as core.DataService with version %s", dsVer.Name(), dsVer.Version())
	logrus.Infof("use %s as core.ServantA with version %s", dsaVer.Name(), dsaVer.Version())
}

func initOSS() {
	var v core.VersionInfo
	if cfg.If("AliOSS") {
		oss, v = storage.MustAliossService()
	} else {
		// default use LocalOSS as object storage service
		oss, v = storage.MustLocalossService()
		logrus.Infof("use default LocalOSS as object storage by version %s", v.Version())
		return
	}
	logrus.Infof("use %s as object storage by version %s", v.Name(), v.Version())
}

func initTsX() {
	var v core.VersionInfo
	ams := newAuthorizationManageService()
	cfg.On(cfg.Actions{
		"Meili": func() {
			ts, v = search.NewMeiliTweetSearchService(ams)
		},
	}, func() {
		// 默认回退 SQL 直查，无需外部搜索引擎
		ts, v = search.NewSqlTweetSearchService(ams)
	})
	logrus.Infof("use %s as tweet search serice by version %s", v.Name(), v.Version())
	ts = search.NewBridgeTweetSearchService(ts)
}
