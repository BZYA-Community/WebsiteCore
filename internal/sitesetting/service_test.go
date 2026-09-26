package sitesetting

import (
	"context"
	"crypto/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
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

func TestGetValuesUsesBootstrapDefaultsWhenSettingsTableIsMissing(t *testing.T) {
	svc := newTestService(t)
	if err := svc.db.Migrator().DropTable(&settingRecord{}); err != nil {
		t.Fatalf("drop settings table: %v", err)
	}
	values, err := svc.GetValues(context.Background())
	if err != nil {
		t.Fatalf("GetValues() with missing settings table: %v", err)
	}
	if !hasValue(values.Items, "web_profile.enable_trends_bar", false, false) {
		t.Fatal("missing bootstrap value for web_profile.enable_trends_bar")
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
	if !profile.AllowPhoneBind {
		t.Fatalf("AllowPhoneBind = false, want bootstrap true")
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
	db := newTestDB(t)
	t.Chdir(t.TempDir())
	if err := os.WriteFile("config.yaml", []byte("App:\n  RunMode: test\nJWT:\n  Secret: sitesetting-test-only\n"), 0600); err != nil {
		t.Fatalf("write test config: %v", err)
	}
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
	if err := db.AutoMigrate(&settingRecord{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	return NewService(db)
}

// Each test gets a fresh database; the supplied DSN is only used to create it.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("WEBSITECORE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("set WEBSITECORE_TEST_POSTGRES_DSN to a PostgreSQL connection with CREATEDB permission")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid WEBSITECORE_TEST_POSTGRES_DSN")
	}
	config.ConnectTimeout = 10 * time.Second
	admin := stdlib.OpenDB(*config)
	t.Cleanup(func() { _ = admin.Close() })
	dbName := "websitecore_test_" + strings.ToLower(rand.Text())
	identifier := pgx.Identifier{dbName}.Sanitize()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+identifier+" TEMPLATE template0"); err != nil {
		t.Fatalf("create isolated PostgreSQL test database: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(ctx, "DROP DATABASE "+identifier+" WITH (FORCE)"); err != nil {
			t.Errorf("drop PostgreSQL test database %s: %v", dbName, err)
		}
	})
	config.Database = dbName
	config.RuntimeParams["search_path"] = "public"
	sqlDB := stdlib.OpenDB(*config)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: "p_", SingularTable: true},
	})
	if err != nil {
		t.Fatalf("open isolated PostgreSQL test database: %v", err)
	}
	return db
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
