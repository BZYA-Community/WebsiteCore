package jinzhu

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/authz"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestWhisperRejectsInvalidParticipants(t *testing.T) {
	s := &messageSrv{}
	for _, msg := range []*ms.Message{
		{Type: ms.MsgTypeWhisper, SenderUserID: 0, ReceiverUserID: 2},
		{Type: ms.MsgTypeWhisper, SenderUserID: 1, ReceiverUserID: -1},
		{Type: ms.MsgTypeWhisper, SenderUserID: 1, ReceiverUserID: 1},
	} {
		if result, err := s.CreateMessage(msg); result != nil || !errors.Is(err, authz.ErrDenied) {
			t.Fatalf("invalid participants accepted: %v, %v", result, err)
		}
	}
}

func TestWhisperTransactions(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set; PostgreSQL concurrent whisper integration test")
	}
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	prefix := "ut_whisper_" + hex.EncodeToString(buf) + "_"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix, SingularTable: true}})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(16)
	models := []any{&dbr.User{}, &dbr.IdentityGroup{}, &dbr.IdentityGroupPermission{}, &dbr.UserIdentityGroup{}, &dbr.Message{}}
	cleanupDB := db
	t.Cleanup(func() {
		for i := len(models) - 1; i >= 0; i-- {
			if err := cleanupDB.Migrator().DropTable(models[i]); err != nil {
				t.Error(err)
			}
		}
	})
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db = db.WithContext(ctx)
	s := &messageSrv{db: db}
	guest := &dbr.IdentityGroup{Key: "guest", Name: "Guest", Builtin: true}
	if err := db.Create(guest).Error; err != nil {
		t.Fatal(err)
	}
	groups := map[string]int64{}
	for i, permission := range []string{authz.MessageInitiate, authz.MessageReply} {
		group := &dbr.IdentityGroup{Key: fmt.Sprintf("capability%d", i), Name: permission}
		if err := db.Create(group).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&dbr.IdentityGroupPermission{GroupID: group.ID, Permission: permission}).Error; err != nil {
			t.Fatal(err)
		}
		groups[permission] = group.ID
	}
	newUser := func(t *testing.T, permissions ...string) *ms.User {
		t.Helper()
		user := &ms.User{Model: &ms.Model{}, Status: ms.UserStatusNormal}
		if err := db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
		for _, permission := range permissions {
			if err := db.Create(&dbr.UserIdentityGroup{UserID: user.ID, GroupID: groups[permission]}).Error; err != nil {
				t.Fatal(err)
			}
		}
		return user
	}
	send := func(sender, receiver *ms.User, content string) (*ms.Message, error) {
		return s.CreateMessage(&ms.Message{SenderUserID: sender.ID, ReceiverUserID: receiver.ID, Type: ms.MsgTypeWhisper, Content: content})
	}
	count := func(t *testing.T) int64 {
		t.Helper()
		var total int64
		if err := db.Model(&dbr.Message{}).Count(&total).Error; err != nil {
			t.Fatal(err)
		}
		return total
	}
	expectDenied := func(t *testing.T, sender, receiver *ms.User, want error) {
		t.Helper()
		before := count(t)
		msg, err := send(sender, receiver, "denied")
		if msg != nil || !errors.Is(err, want) || count(t) != before {
			t.Fatalf("failed send changed history: message=%v, error=%v, want=%v", msg, err, want)
		}
	}

	t.Run("concurrent first messages and reply", func(t *testing.T) {
		sender := newUser(t, authz.MessageInitiate, authz.MessageReply)
		receiver := newUser(t, authz.MessageReply)
		const workers = 16
		start, results := make(chan struct{}), make(chan error, workers)
		for i := 0; i < workers; i++ {
			go func(i int) {
				<-start
				_, err := send(sender, receiver, fmt.Sprintf("first %d", i))
				results <- err
			}(i)
		}
		before := count(t)
		close(start)
		success, pending := 0, 0
		for i := 0; i < workers; i++ {
			switch err := <-results; {
			case err == nil:
				success++
			case errors.Is(err, core.ErrWhisperOnePending):
				pending++
			default:
				t.Errorf("concurrent send: %v", err)
			}
		}
		if success != 1 || pending != workers-1 || count(t) != before+1 {
			t.Fatalf("success=%d pending=%d; expected one persisted initial message", success, pending)
		}
		if _, err := send(receiver, sender, "reply"); err != nil {
			t.Fatal(err)
		}
		if _, err := send(sender, receiver, "after reply"); err != nil {
			t.Fatal(err)
		}
		// A previously loaded sender must not retain a revoked permission.
		if err := dbr.LoadUserIdentities(db, sender); err != nil || !sender.HasPermission(authz.MessageReply) {
			t.Fatalf("load sender policy: %v", err)
		}
		if err := db.Where("user_id = ? AND group_id = ?", sender.ID, groups[authz.MessageReply]).Delete(&dbr.UserIdentityGroup{}).Error; err != nil {
			t.Fatal(err)
		}
		expectDenied(t, sender, receiver, authz.ErrDenied)
	})

	t.Run("permissions status and operator boundaries", func(t *testing.T) {
		initiator, replier, recipient := newUser(t, authz.MessageInitiate), newUser(t, authz.MessageReply), newUser(t)
		expectDenied(t, replier, recipient, authz.ErrDenied)
		expectDenied(t, recipient, initiator, authz.ErrDenied)
		if _, err := send(initiator, replier, "start"); err != nil {
			t.Fatal(err)
		}
		if _, err := send(replier, initiator, "reply"); err != nil {
			t.Fatal(err)
		}
		expectDenied(t, initiator, replier, authz.ErrDenied)
		operator := newUser(t)
		if err := db.Model(&dbr.User{}).Where("id = ?", operator.ID).Update("is_operator", true).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := send(operator, recipient, "operator start"); err != nil {
			t.Fatal(err)
		}
		expectDenied(t, operator, recipient, core.ErrWhisperOnePending)
		if err := db.Model(&dbr.User{}).Where("id = ?", operator.ID).Update("status", ms.UserStatusClosed).Error; err != nil {
			t.Fatal(err)
		}
		expectDenied(t, operator, recipient, authz.ErrDenied)
		if err := db.Model(&dbr.User{}).Where("id = ?", recipient.ID).Update("status", ms.UserStatusClosed).Error; err != nil {
			t.Fatal(err)
		}
		expectDenied(t, initiator, recipient, authz.ErrDenied)
		if err := db.Delete(replier).Error; err != nil {
			t.Fatal(err)
		}
		expectDenied(t, initiator, replier, authz.ErrDenied)
		expectDenied(t, replier, initiator, authz.ErrDenied)
		expectDenied(t, initiator, &ms.User{Model: &ms.Model{ID: 999999}}, authz.ErrDenied)
	})

	t.Run("failed insert leaves no pending conversation", func(t *testing.T) {
		sender, recipient := newUser(t, authz.MessageInitiate), newUser(t)
		if err := db.Exec("ALTER TABLE " + prefix + "message ADD CONSTRAINT reject_content CHECK (content <> 'reject')").Error; err != nil {
			t.Fatal(err)
		}
		before := count(t)
		msg, err := send(sender, recipient, "reject")
		if err == nil || msg != nil || count(t) != before {
			t.Fatalf("failed insert leaked message: %v, %v", msg, err)
		}
		if _, err := send(sender, recipient, "valid retry"); err != nil {
			t.Fatalf("failed transaction left a pending message or lock: %v", err)
		}
	})
}
