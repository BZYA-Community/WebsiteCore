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
	loggerSetting     *loggerConf
	loggerFileSetting *loggerFileConf
	loggerOtlpSetting *loggerOtlponf
	sentrySetting     *sentryConf
	redisSetting      *redisConf

	PyroscopeSetting        *pyroscopeConf
	DatabaseSetting         *databaseConf
	PostgresSetting         *postgresConf
	PprofServerSetting      *httpServerConf
	MetricsServerSetting    *httpServerConf
	WebServerSetting        *httpServerConf
	FrontendWebSetting      *httpServerConf
	DocsServerSetting       *httpServerConf
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
		"FrontendWebServer": &FrontendWebSetting,
		"DocsServer":        &DocsServerSetting,
		"CacheIndex":        &CacheIndexSetting,
		"SimpleCacheIndex":  &SimpleCacheIndexSetting,
		"BigCacheIndex":     &BigCacheIndexSetting,
		"RedisCacheIndex":   &RedisCacheIndexSetting,
		"SmsJuhe":           &SmsJuheSetting,
		"Pyroscope":         &PyroscopeSetting,
		"Sentry":            &sentrySetting,
		"Logger":            &loggerSetting,
		"LoggerFile":        &loggerFileSetting,
		"LoggerOtlp":        &loggerOtlpSetting,
		"Database":          &DatabaseSetting,
		"Postgres":          &PostgresSetting,
		"TweetSearch":       &TweetSearchSetting,
		"Meili":             &MeiliSetting,
		"Redis":             &redisSetting,
		"JWT":               &JWTSetting,
		"AdminSettings":     &AdminSettingsSetting,
		"ObjectStorage":     &ObjectStorage,
		"AliOSS":            &AliOSSSetting,
		"LocalOSS":          &LocalOSSSetting,
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
	}
	// default use LocalOSS
	if !LocalOSSSetting.Secure {
		uri = "http://"
	}
	return uri + LocalOSSSetting.Domain + "/oss/" + LocalOSSSetting.Bucket + "/"
}

func RunMode() string {
	return AppSetting.RunMode
}

func UseSentryGin() bool {
	return cfg.If("Sentry") && sentrySetting.AttachGin
}
