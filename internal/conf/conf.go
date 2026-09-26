// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package conf

import (
	"errors"
	"fmt"
	"time"

	"github.com/alimy/tryst/cfg"
)

var (
	loggerSetting            *loggerConf
	loggerFileSetting        *loggerFileConf
	loggerMeiliSetting       *loggerMeiliConf
	loggerOpenObserveSetting *loggerOpenObserveConf
	loggerOtlpSetting        *loggerOtlponf
	sentrySetting            *sentryConf
	redisSetting             *redisConf

	PyroscopeSetting        *pyroscopeConf
	DatabaseSetting         *databaseConf
	MysqlSetting            *mysqlConf
	PostgresSetting         *postgresConf
	PprofServerSetting      *httpServerConf
	MetricsServerSetting    *httpServerConf
	WebServerSetting        *httpServerConf
	AdminServerSetting      *httpServerConf
	SpaceXServerSetting     *httpServerConf
	BotServerSetting        *httpServerConf
	LocalossServerSetting   *httpServerConf
	FrontendWebSetting      *httpServerConf
	DocsServerSetting       *httpServerConf
	MobileServerSetting     *grpcServerConf
	AppSetting              *appConf
	CacheSetting            *cacheConf
	EventManagerSetting     *eventManagerConf
	MetricManagerSetting    *metricManagerConf
	JobManagerSetting       *jobManagerConf
	CacheIndexSetting       *cacheIndexConf
	SimpleCacheIndexSetting *simpleCacheIndexConf
	BigCacheIndexSetting    *bigCacheIndexConf
	RedisCacheIndexSetting  *redisCacheIndexConf
	SmsJuheSetting          *smsJuheConf
	TweetSearchSetting      *tweetSearchConf
	MeiliSetting            *meiliConf
	ObjectStorage           *objectStorageConf
	AliOSSSetting           *aliOSSConf
	COSSetting              *cosConf
	HuaweiOBSSetting        *huaweiOBSConf
	MinIOSetting            *minioConf
	S3Setting               *s3Conf
	LocalOSSSetting         *localossConf
	JWTSetting              *jwtConf
	AdminSettingsSetting    *adminSettingsConf
	WebProfileSetting       *WebProfileConf
	AuditSetting            *AuditConf
	OperatorSetting         *OperatorConf
)

func setupSetting(suite []string, noDefault bool) error {
	vp, err := newViper()
	if err != nil {
		return err
	}

	// initialize features configure
	ss, kv := featuresInfoFrom(vp, "Features")
	cfg.Initial(ss, kv)
	if len(suite) > 0 {
		cfg.Use(suite, noDefault)
	}

	objects := map[string]any{
		"App":               &AppSetting,
		"Cache":             &CacheSetting,
		"EventManager":      &EventManagerSetting,
		"MetricManager":     &MetricManagerSetting,
		"JobManager":        &JobManagerSetting,
		"PprofServer":       &PprofServerSetting,
		"MetricsServer":     &MetricsServerSetting,
		"WebServer":         &WebServerSetting,
		"AdminServer":       &AdminServerSetting,
		"SpaceXServer":      &SpaceXServerSetting,
		"BotServer":         &BotServerSetting,
		"LocalossServer":    &LocalossServerSetting,
		"FrontendWebServer": &FrontendWebSetting,
		"DocsServer":        &DocsServerSetting,
		"MobileServer":      &MobileServerSetting,
		"CacheIndex":        &CacheIndexSetting,
		"SimpleCacheIndex":  &SimpleCacheIndexSetting,
		"BigCacheIndex":     &BigCacheIndexSetting,
		"RedisCacheIndex":   &RedisCacheIndexSetting,
		"SmsJuhe":           &SmsJuheSetting,
		"Pyroscope":         &PyroscopeSetting,
		"Sentry":            &sentrySetting,
		"Logger":            &loggerSetting,
		"LoggerFile":        &loggerFileSetting,
		"LoggerMeili":       &loggerMeiliSetting,
		"LoggerOpenObserve": &loggerOpenObserveSetting,
		"LoggerOtlp":        &loggerOtlpSetting,
		"Database":          &DatabaseSetting,
		"MySQL":             &MysqlSetting,
		"Postgres":          &PostgresSetting,
		"TweetSearch":       &TweetSearchSetting,
		"Meili":             &MeiliSetting,
		"Redis":             &redisSetting,
		"JWT":               &JWTSetting,
		"AdminSettings":     &AdminSettingsSetting,
		"ObjectStorage":     &ObjectStorage,
		"AliOSS":            &AliOSSSetting,
		"COS":               &COSSetting,
		"HuaweiOBS":         &HuaweiOBSSetting,
		"MinIO":             &MinIOSetting,
		"LocalOSS":          &LocalOSSSetting,
		"S3":                &S3Setting,
		"WebProfile":        &WebProfileSetting,
		"Audit":             &AuditSetting,
		"Operator":          &OperatorSetting,
	}
	for k, v := range objects {
		err := vp.UnmarshalKey(k, v)
		if err != nil {
			return err
		}
	}

	// yaml 缺省 Audit 段时保持默认开启内容审核
	if AuditSetting == nil {
		AuditSetting = &AuditConf{Enabled: true}
	}

	CacheSetting.CientSideCacheExpire *= time.Second
	EventManagerSetting.MaxIdleTime *= time.Second
	MetricManagerSetting.MaxIdleTime *= time.Second
	JWTSetting.Expire *= time.Second
	SimpleCacheIndexSetting.CheckTickDuration *= time.Second
	SimpleCacheIndexSetting.ExpireTickDuration *= time.Second
	BigCacheIndexSetting.ExpireInSecond *= time.Second
	RedisCacheIndexSetting.ExpireInSecond *= time.Second
	redisSetting.ConnWriteTimeout *= time.Second

	// Validate critical security settings
	if JWTSetting.Secret == "" {
		return errors.New("JWT Secret is not set. Generate one with: openssl rand -base64 32\nSet it in custom/config.yaml under JWT.Secret")
	}

	return nil
}

// Initial 初始化配置/日志/Sentry，失败时返回 error 而不是直接杀进程；
// 是否退出由调用方（cmd 层）决定。
func Initial(suite []string, noDefault bool) error {
	if err := setupSetting(suite, noDefault); err != nil {
		return fmt.Errorf("init.setupSetting err: %w", err)
	}
	setupLogger()
	initSentry()
	return nil
}

func GetOssDomain() string {
	uri := "https://"
	if cfg.If("AliOSS") {
		return uri + AliOSSSetting.Domain + "/"
	} else if cfg.If("COS") {
		return uri + COSSetting.Domain + "/"
	} else if cfg.If("HuaweiOBS") {
		return uri + HuaweiOBSSetting.Domain + "/"
	} else if cfg.If("MinIO") {
		if !MinIOSetting.Secure {
			uri = "http://"
		}
		return uri + MinIOSetting.Domain + "/" + MinIOSetting.Bucket + "/"
	} else if cfg.If("S3") {
		if !S3Setting.Secure {
			uri = "http://"
		}
		// TODO: will not work well need test in real world
		return uri + S3Setting.Domain + "/" + S3Setting.Bucket + "/"
	} else if cfg.If("LocalOSS") {
		if !LocalOSSSetting.Secure {
			uri = "http://"
		}
		return uri + LocalOSSSetting.Domain + "/oss/" + LocalOSSSetting.Bucket + "/"
	}
	return uri + AliOSSSetting.Domain + "/"
}

func RunMode() string {
	return AppSetting.RunMode
}

func UseSentryGin() bool {
	return cfg.If("Sentry") && sentrySetting.AttachGin
}
