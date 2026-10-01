package jinzhu

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestGeneratePhoneCaptcha(t *testing.T) {
	for i := 0; i < 100; i++ {
		got, err := generatePhoneCaptcha()
		if err != nil {
			t.Fatalf("generatePhoneCaptcha() error = %v", err)
		}
		if len(got) != 6 {
			t.Fatalf("generatePhoneCaptcha() = %q, want six digits", got)
		}
		n, err := strconv.Atoi(got)
		if err != nil || n < 100000 || n > 999999 {
			t.Fatalf("generatePhoneCaptcha() = %q, want 100000..999999", got)
		}
	}
}

func TestVerifyPhoneCaptchaConsumesAttemptsAndSuccess(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set, skip persistence test")
	}
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}
	prefix := "ut_captcha_" + hex.EncodeToString(buf) + "_"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix, SingularTable: true}})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	if err := db.AutoMigrate(&dbr.Captcha{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Migrator().DropTable(&dbr.Captcha{}) })

	record := &dbr.Captcha{Phone: "13800000000", Captcha: "123456", ExpiredOn: time.Now().Add(time.Minute).Unix()}
	if _, err := record.Create(db); err != nil {
		t.Fatalf("create captcha: %v", err)
	}
	srv := &securitySrv{db: db}
	if ok, err := srv.VerifyPhoneCaptcha(record.Phone, "000000", 3); err != nil || ok {
		t.Fatalf("wrong code = (%v, %v), want (false, nil)", ok, err)
	}
	current, err := (&dbr.Captcha{Phone: record.Phone}).Get(db)
	if err != nil || current.UseTimes != 1 {
		t.Fatalf("wrong attempt use_times = %d, err = %v, want 1", current.UseTimes, err)
	}
	if ok, err := srv.VerifyPhoneCaptcha(record.Phone, record.Captcha, 3); err != nil || !ok {
		t.Fatalf("correct code = (%v, %v), want (true, nil)", ok, err)
	}
	if ok, err := srv.VerifyPhoneCaptcha(record.Phone, record.Captcha, 3); !errors.Is(err, core.ErrPhoneCaptchaMaxAttempts) || ok {
		t.Fatalf("reused code = (%v, %v), want (false, max-attempts)", ok, err)
	}
}
