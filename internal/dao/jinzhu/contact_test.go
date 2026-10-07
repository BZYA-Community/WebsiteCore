package jinzhu

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type contactPhoneSender struct {
	code string
	ttl  time.Duration
}

func (s *contactPhoneSender) SendPhoneCaptcha(_ string, code string, ttl time.Duration) error {
	s.code, s.ttl = code, ttl
	return nil
}

func TestContactNormalization(t *testing.T) {
	for _, address := range []string{"name <user@example.test>", "user@example.test\r\nBcc: x@example.test", "not an address"} {
		if _, err := normalizeContact("email", address); err == nil {
			t.Fatalf("accepted %q", address)
		}
	}
	if got, err := normalizeContact("email", "User@Example.TEST"); err != nil || got != "user@example.test" {
		t.Fatal(got, err)
	}
	if got, err := normalizeContact("phone", "+8613800000000"); err != nil || got != "13800000000" {
		t.Fatal(got, err)
	}
	if _, err := normalizeContact("phone", "123456"); err == nil {
		t.Fatal("accepted invalid phone")
	}
}

func TestContactVerificationTransactions(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}
	var random [6]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	prefix := "ut_contact_" + hex.EncodeToString(random[:]) + "_"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix, SingularTable: true}})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	models := []any{&dbr.User{}, &dbr.ContactVerification{}, &dbr.OperationLog{}}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for i := len(models) - 1; i >= 0; i-- {
			if err := db.Migrator().DropTable(models[i]); err != nil {
				t.Log(err)
			}
		}
	})
	if err := db.Exec("CREATE UNIQUE INDEX " + prefix + "email ON " + prefix + "user(lower(email)) WHERE email <> '' AND is_del=0").Error; err != nil {
		t.Fatal(err)
	}
	old := conf.AccountVerifySetting
	conf.AccountVerifySetting = &conf.AccountVerifyConf{Mode: "email", CodeTTLSeconds: 300, CooldownSeconds: 60, MaxAttempts: 5, MaxDailySends: 10, Aliyun: conf.AliyunMailConf{ClientID: "test", ClientSecret: "test", Sender: "test@example.test"}}
	t.Cleanup(func() { conf.AccountVerifySetting = old })
	codes := map[string]string{}
	s := &securitySrv{db: db, codeSecret: "test-only-hmac-secret", sendEmail: func(_ context.Context, address, code string, _ time.Duration) error {
		codes[address] = code
		return nil
	}}
	ctx := context.Background()
	next := 0
	newUser := func() *dbr.User {
		next++
		u := &dbr.User{Model: &dbr.Model{}, Username: fmt.Sprintf("user%d", next), Status: dbr.UserStatusNormal}
		if err := db.Create(u).Error; err != nil {
			t.Fatal(err)
		}
		return u
	}
	send := func(u *dbr.User, address string) {
		t.Helper()
		if err := s.SendContactCode(ctx, u.ID, "email", address, "127.0.0.1"); err != nil {
			t.Fatal(err)
		}
	}
	u := newUser()
	send(u, "member@example.test")
	if err := s.SendContactCode(ctx, u.ID, "email", "different@example.test", "127.0.0.1"); !errors.Is(err, core.ErrContactRateLimited) {
		t.Fatalf("cooldown bypass: %v", err)
	}
	if err := s.SendContactCode(ctx, newUser().ID, "email", "member@example.test", "127.0.0.1"); !errors.Is(err, core.ErrContactRateLimited) {
		t.Fatalf("address cooldown bypass: %v", err)
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.VerifyContactCode(ctx, u.ID, "email", "member@example.test", codes["member@example.test"])
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, core.ErrContactCode) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("code consumed %d times", successes.Load())
	}
	var bound dbr.User
	if err := db.First(&bound, u.ID).Error; err != nil || !bound.ContactVerified() {
		t.Fatalf("verification not applied: %v", err)
	}
	if err := s.SendContactCode(ctx, newUser().ID, "email", "member@example.test", "127.0.0.1"); !errors.Is(err, core.ErrContactInUse) {
		t.Fatalf("duplicate address accepted: %v", err)
	}
	if err := s.VerifyContactCode(ctx, u.ID, "phone", "13800000000", "123456"); !errors.Is(err, core.ErrContactDisabled) {
		t.Fatalf("inactive channel accepted: %v", err)
	}
	w := newUser()
	send(w, "wrong@example.test")
	wrong := "000000"
	if codes["wrong@example.test"] == wrong {
		wrong = "111111"
	}
	for range 5 {
		if err := s.VerifyContactCode(ctx, w.ID, "email", "wrong@example.test", wrong); !errors.Is(err, core.ErrContactCode) {
			t.Fatal(err)
		}
	}
	if err := s.VerifyContactCode(ctx, w.ID, "email", "wrong@example.test", codes["wrong@example.test"]); !errors.Is(err, core.ErrContactCode) {
		t.Fatalf("exhausted code accepted: %v", err)
	}
	expired := newUser()
	send(expired, "expired@example.test")
	if err := db.Model(&dbr.ContactVerification{}).Where("user_id = ?", expired.ID).Update("expires_on", time.Now().Unix()-1).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.VerifyContactCode(ctx, expired.ID, "email", "expired@example.test", codes["expired@example.test"]); !errors.Is(err, core.ErrContactCode) {
		t.Fatalf("expired code accepted: %v", err)
	}
	failed := newUser()
	s.sendEmail = func(_ context.Context, address, code string, _ time.Duration) error {
		codes[address] = code
		return errors.New("test delivery failure")
	}
	if err := s.SendContactCode(ctx, failed.ID, "email", "failed@example.test", "127.0.0.1"); err == nil {
		t.Fatal("ignored delivery failure")
	}
	if err := s.VerifyContactCode(ctx, failed.ID, "email", "failed@example.test", codes["failed@example.test"]); !errors.Is(err, core.ErrContactCode) {
		t.Fatalf("failed delivery code accepted: %v", err)
	}
	if err := s.SendContactCode(ctx, failed.ID, "email", "failed@example.test", "127.0.0.1"); !errors.Is(err, core.ErrContactRateLimited) {
		t.Fatalf("failure reset cooldown: %v", err)
	}
	s.sendEmail = func(_ context.Context, address, code string, _ time.Duration) error {
		codes[address] = code
		return nil
	}
	rollback := newUser()
	send(rollback, "rollback@example.test")
	const callback = "test:contact_log_failure"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if _, ok := tx.Statement.Dest.(*dbr.OperationLog); ok {
			tx.AddError(errors.New("injected log failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.VerifyContactCode(ctx, rollback.ID, "email", "rollback@example.test", codes["rollback@example.test"]); err == nil {
		t.Fatal("ignored operation log failure")
	}
	if err := db.Callback().Create().Remove(callback); err != nil {
		t.Fatal(err)
	}
	var unchanged dbr.User
	if err := db.First(&unchanged, rollback.ID).Error; err != nil || unchanged.ContactVerified() {
		t.Fatalf("failed transaction bound contact: %v", err)
	}
	if err := s.VerifyContactCode(ctx, rollback.ID, "email", "rollback@example.test", codes["rollback@example.test"]); err != nil {
		t.Fatalf("rollback consumed code: %v", err)
	}
	oldSMS := conf.SmsJuheSetting
	t.Cleanup(func() { conf.SmsJuheSetting = oldSMS })
	conf.SmsJuheSetting = nil
	if err := json.Unmarshal([]byte(`{"Key":"synthetic","TplID":"synthetic"}`), &conf.SmsJuheSetting); err != nil {
		t.Fatal(err)
	}
	conf.AccountVerifySetting.Mode = "phone"
	phone := &contactPhoneSender{}
	s.phoneVerify = phone
	phoneUser := newUser()
	if err := s.SendContactCode(ctx, phoneUser.ID, "phone", "+8613800000000", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if phone.ttl != 5 || len(phone.code) != 6 {
		t.Fatal("legacy provider did not receive its expected minute-based TTL")
	}
	if err := s.VerifyContactCode(ctx, phoneUser.ID, "phone", "13800000000", phone.code); err != nil {
		t.Fatal(err)
	}
	var verifiedPhone dbr.User
	if err := db.First(&verifiedPhone, phoneUser.ID).Error; err != nil || !verifiedPhone.ContactVerified() {
		t.Fatalf("phone mode did not verify account: %v", err)
	}
	if bound.ContactVerified() {
		t.Fatal("email-only account treated as verified in phone mode")
	}
	if err := s.SendContactCode(ctx, phoneUser.ID, "email", "disabled@example.test", "127.0.0.1"); !errors.Is(err, core.ErrContactDisabled) {
		t.Fatalf("email channel enabled in phone mode: %v", err)
	}
}
