// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package storage

import (
	"path/filepath"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/alimy/tryst/cfg"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/sirupsen/logrus"
)

func MustAliossService() (core.ObjectStorageService, core.VersionInfo) {
	client, err := oss.New(conf.AliOSSSetting.Endpoint, conf.AliOSSSetting.AccessKeyID, conf.AliOSSSetting.AccessKeySecret)
	if err != nil {
		logrus.Fatalf("storage.MustAliossService create client err: %s", err)
	}

	bucket, err := client.Bucket(conf.AliOSSSetting.Bucket)
	if err != nil {
		logrus.Fatalf("storage.MustAliossService create bucket err: %s", err)
	}

	domain := conf.GetOssDomain()
	var cs core.OssCreateService
	if cfg.If("OSS:TempDir") {
		cs = &aliossCreateTempDirServant{
			bucket:  bucket,
			domain:  domain,
			tempDir: conf.ObjectStorage.TempDirSlash(),
		}
		logrus.Debugln("use OSS:TempDir feature")
	} else if cfg.If("OSS:Retention") {
		cs = &aliossCreateRetentionServant{
			bucket:          bucket,
			domain:          domain,
			retainInDays:    time.Duration(conf.ObjectStorage.RetainInDays) * time.Hour * 24,
			retainUntilDate: time.Date(2049, time.December, 1, 12, 0, 0, 0, time.UTC),
		}
		logrus.Debugln("use OSS:Retention feature")
	} else {
		cs = &aliossCreateServant{
			bucket: bucket,
			domain: domain,
		}
		logrus.Debugln("use OSS:Direct feature")
	}

	obj := &aliossServant{
		OssCreateService: cs,
		bucket:           bucket,
		domain:           conf.GetOssDomain(),
	}
	return obj, obj
}

func MustLocalossService() (core.ObjectStorageService, core.VersionInfo) {
	savePath, err := filepath.Abs(conf.LocalOSSSetting.SavePath)
	if err != nil {
		logrus.Fatalf("storage.MustLocalossService get localOSS save path err: %s", err)
	}

	domain := conf.GetOssDomain()
	savePath = savePath + "/" + conf.LocalOSSSetting.Bucket + "/"
	var cs core.OssCreateService
	if cfg.If("OSS:TempDir") {
		cs = &localossCreateTempDirServant{
			savePath: savePath,
			domain:   domain,
			tempDir:  conf.ObjectStorage.TempDirSlash(),
		}
		logrus.Debugln("use OSS:TempDir feature")
	} else {
		cs = &localossCreateServant{
			savePath: savePath,
			domain:   domain,
		}
		logrus.Debugln("use OSS:Direct feature")
	}

	obj := &localossServant{
		OssCreateService: cs,
		savePath:         savePath,
		domain:           domain,
	}
	return obj, obj
}
