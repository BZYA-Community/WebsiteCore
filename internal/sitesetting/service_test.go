package sitesetting

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestGetProfileUsesBootstrapDefaultsWhenNoOverride(t *testing.T) {
	svc := newTestService(t)

	profile, err := svc.GetProfile(context.Background())
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if !profile.AllowUserRegister {
		t.Fatalf("AllowUserRegister = false, want bootstrap true")
	}
	if profile.DefaultTweetVisibility != "friend" {
		t.Fatalf("DefaultTweetVisibility = %q, want friend", profile.DefaultTweetVisibility)
	}
	if profile.CopyrightRight != "fallback-right" {
		t.Fatalf("CopyrightRight = %q, want fallback-right", profile.CopyrightRight)
	}
}

func TestUpdateEditableProfilePersistsOnlyEditableKeys(t *testing.T) {
	svc := newTestService(t)

	profile, err := svc.UpdateEditableProfile(context.Background(), EditableProfile{
		EnableTrendsBar:         true,
		AllowTweetAttachment:    false,
		AllowTweetVideo:         false,
		DefaultTweetMaxLength:   1200,
		TweetWebEllipsisSize:    300,
		TweetMobileEllipsisSize: 200,
		DefaultTweetVisibility:  "public",
		DefaultMsgLoopInterval:  3000,
		CopyrightTop:            "top",
		CopyrightLeft:           "left",
		CopyrightLeftLink:       "https://left.example.com",
		CopyrightRight:          "right",
		CopyrightRightLink:      "https://right.example.com",
	})
	if err != nil {
		t.Fatalf("UpdateEditableProfile() error = %v", err)
	}
	if !profile.AllowUserRegister {
		t.Fatalf("AllowUserRegister = false, want bootstrap true")
	}
	if profile.AllowPhoneBind || profile.AccountVerifyMode != "email" {
		t.Fatal("legacy bootstrap flag enabled phone binding in email mode")
	}
	if profile.DefaultTweetVisibility != "public" {
		t.Fatalf("DefaultTweetVisibility = %q, want public", profile.DefaultTweetVisibility)
	}
	values, err := svc.GetValues(context.Background())
	if err != nil {
		t.Fatalf("GetValues() error = %v", err)
	}
	if !hasValue(values.Items, "web_profile.enable_trends_bar", true, false) {
		t.Fatalf("web_profile.enable_trends_bar override not found")
	}
}

func TestSaveValuesEncryptsSecretsAtRest(t *testing.T) {
	svc := newTestService(t)
	conf.AdminSettingsSetting.EncryptionKey = "bootstrap-test-encryption-key"
	svc.codec = newSecretCodec()

	_, err := svc.SaveValues(context.Background(), []web.AdminSettingValueInput{{
		Key:   "meili.api_key",
		Value: []byte(`"top-secret-key"`),
	}})
	if err != nil {
		t.Fatalf("SaveValues() error = %v", err)
	}
	var record settingRecord
	if err := svc.db.WithContext(context.Background()).First(&record, "key = ?", "meili.api_key").Error; err != nil {
		t.Fatalf("load record error = %v", err)
	}
	if !record.IsEncrypted {
		t.Fatalf("IsEncrypted = false, want true")
	}
	if strings.Contains(record.Value, "top-secret-key") {
		t.Fatalf("record.Value stored plaintext = %q", record.Value)
	}
}

func TestAliyunSMSOverridesEncryptCredentialsAndRequireRestart(t *testing.T) {
	svc := newTestService(t)
	previousSMS, previousMode := *conf.SmsAliyunSetting, conf.AccountVerifySetting.Mode
	t.Cleanup(func() { *conf.SmsAliyunSetting, conf.AccountVerifySetting.Mode = previousSMS, previousMode })
	conf.AdminSettingsSetting.EncryptionKey = "bootstrap-test-encryption-key"
	svc.codec = newSecretCodec()
	ctx := context.Background()
	_, err := svc.SaveValues(ctx, []web.AdminSettingValueInput{
		{Key: "sms_aliyun.access_key_id", Value: []byte(`"synthetic-sms-id"`)},
		{Key: "sms_aliyun.access_key_secret", Value: []byte(`"synthetic-sms-secret"`)},
		{Key: "sms_aliyun.sign_name", Value: []byte(`"测试签名"`)},
		{Key: "sms_aliyun.template_code", Value: []byte(`"SMS_synthetic"`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if conf.SmsConfigured() {
		t.Fatal("SMS settings took effect before restart")
	}
	for key, plaintext := range map[string]string{"sms_aliyun.access_key_id": "synthetic-sms-id", "sms_aliyun.access_key_secret": "synthetic-sms-secret"} {
		var record settingRecord
		if err := svc.db.WithContext(ctx).First(&record, "key = ?", key).Error; err != nil {
			t.Fatal(err)
		}
		if !record.IsEncrypted || !strings.HasPrefix(record.Value, encryptedValuePrefix) || strings.Contains(record.Value, plaintext) {
			t.Fatalf("SMS credential was not encrypted: %s", key)
		}
	}
	values, err := svc.GetValues(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !values.HasPendingRestart {
		t.Fatal("SMS overrides did not report pending restart")
	}
	count := 0
	for _, item := range values.Items {
		if !strings.HasPrefix(item.Key, "sms_aliyun.") {
			continue
		}
		count++
		if !item.PendingRestart {
			t.Fatalf("SMS setting did not require restart: %s", item.Key)
		}
		if item.Key == "sms_aliyun.access_key_id" || item.Key == "sms_aliyun.access_key_secret" {
			if !item.Configured || item.Value != nil || item.EffectiveValue != nil {
				t.Fatalf("SMS credential leaked in values: %s", item.Key)
			}
		}
	}
	if count != 4 {
		t.Fatalf("SMS override count = %d, want 4", count)
	}
	schema, err := svc.GetSchema()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range schema.Items {
		if item.Key == "sms_aliyun.access_key_id" || item.Key == "sms_aliyun.access_key_secret" {
			if !item.Secret || item.BootstrapValue != nil {
				t.Fatalf("SMS credential leaked in schema: %s", item.Key)
			}
		}
	}
	if err := svc.ApplyPersistedOverrides(ctx); err != nil {
		t.Fatal(err)
	}
	conf.AccountVerifySetting.Mode = "phone"
	sms := conf.SmsAliyunSetting
	if !conf.VerificationAvailable() || sms.AccessKeyID != "synthetic-sms-id" || sms.AccessKeySecret != "synthetic-sms-secret" || sms.SignName != "测试签名" || sms.TemplateCode != "SMS_synthetic" {
		t.Fatal("persisted SMS settings did not become effective after restart")
	}
}

func TestRestartRequiredValuesReportPendingRestart(t *testing.T) {
	svc := newTestService(t)

	resp, err := svc.SaveValues(context.Background(), []web.AdminSettingValueInput{{
		Key:   "meili.host",
		Value: []byte(`"pending-restart:7700"`),
	}})
	if err != nil {
		t.Fatalf("SaveValues() error = %v", err)
	}
	if !resp.HasPendingRestart {
		t.Fatal("HasPendingRestart = false, want true")
	}
	if !hasValue(resp.Items, "meili.host", "pending-restart:7700", true) {
		t.Fatalf("meili.host pending_restart not reported")
	}
}

func TestSaveValuesRejectsInvalidSingleIntUpdate(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.SaveValues(context.Background(), []web.AdminSettingValueInput{{
		Key:   "web_profile.default_tweet_max_length",
		Value: []byte(`120`),
	}})
	if err == nil {
		t.Fatal("SaveValues() error = nil, want invalid params error")
	}
}

func TestSaveValuesAcceptsCoupledProfileUpdate(t *testing.T) {
	svc := newTestService(t)

	resp, err := svc.SaveValues(context.Background(), []web.AdminSettingValueInput{
		{Key: "web_profile.default_tweet_max_length", Value: []byte(`120`)},
		{Key: "web_profile.tweet_web_ellipsis_size", Value: []byte(`120`)},
		{Key: "web_profile.tweet_mobile_ellipsis_size", Value: []byte(`120`)},
	})
	if err != nil {
		t.Fatalf("SaveValues() error = %v", err)
	}
	if !hasValue(resp.Items, "web_profile.default_tweet_max_length", 120, false) {
		t.Fatalf("web_profile.default_tweet_max_length override not found")
	}
	if conf.WebProfileSetting.DefaultTweetMaxLength != 120 {
		t.Fatalf("DefaultTweetMaxLength = %d, want 120", conf.WebProfileSetting.DefaultTweetMaxLength)
	}
}

func TestBootstrapAppliesPersistedOverrides(t *testing.T) {
	svc := newTestService(t)
	conf.AdminSettingsSetting.EncryptionKey = "bootstrap-test-encryption-key"
	svc.codec = newSecretCodec()

	_, err := svc.SaveValues(context.Background(), []web.AdminSettingValueInput{{
		Key:   "web_profile.enable_trends_bar",
		Value: []byte(`true`),
	}})
	if err != nil {
		t.Fatalf("SaveValues() error = %v", err)
	}
	conf.WebProfileSetting.EnableTrendsBar = false
	Bootstrap(context.Background(), svc.db)
	if !conf.WebProfileSetting.EnableTrendsBar {
		t.Fatal("Bootstrap() did not apply persisted web_profile.enable_trends_bar override")
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	// 测试不依赖仓库根目录的 config.yaml(干净检出/CI 上没有该文件):
	// 切到临时目录并放置一份最小配置, 未覆盖的配置项与内嵌默认配置合并。
	dir := t.TempDir()
	minimalConf := "# 单元测试最小配置, 其余项取内嵌默认值\n" +
		"App:\n  RunMode: test\n" +
		"JWT:\n  Secret: \"unit-test-only-secret\"\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(minimalConf), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", filepath.Join(dir, "config.yaml"), err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir(%q) error = %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
	if err := conf.Initial(nil, false); err != nil {
		t.Fatalf("conf.Initial(nil, false) error = %v", err)
	}
	conf.WebProfileSetting = &conf.WebProfileConf{
		EnableTrendsBar:         false,
		AllowTweetAttachment:    true,
		AllowTweetVideo:         true,
		AllowUserRegister:       true,
		AllowPhoneBind:          true,
		DefaultTweetMaxLength:   2000,
		TweetWebEllipsisSize:    400,
		TweetMobileEllipsisSize: 300,
		DefaultTweetVisibility:  "friend",
		DefaultMsgLoopInterval:  5000,
		CopyrightTop:            "fallback-top",
		CopyrightLeft:           "fallback-left",
		CopyrightLeftLink:       "",
		CopyrightRight:          "fallback-right",
		CopyrightRightLink:      "https://fallback.example.com",
	}
	bootstrapConfig = nil
	// 项目仅支持 PostgreSQL: 持久化测试需要一个真实 PG 实例，
	// 通过 TEST_POSTGRES_DSN 提供(本地可指向 docker-compose.dev.yml 的 postgres)，
	// 未设置时跳过这些用例。
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set, skip sitesetting persistence test")
	}
	// 随机表前缀隔离并发/重复运行的测试数据
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}
	tablePrefix := "ut_" + hex.EncodeToString(buf) + "_"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: tablePrefix, SingularTable: true}})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := db.AutoMigrate(&settingRecord{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Migrator().DropTable(&settingRecord{}); err != nil {
			t.Logf("DropTable() error = %v", err)
		}
	})
	return NewService(db)
}

func hasValue(items []web.AdminSettingValue, key string, expected any, pending bool) bool {
	for _, item := range items {
		if item.Key != key {
			continue
		}
		return item.Value == expected && item.PendingRestart == pending
	}
	return false
}
