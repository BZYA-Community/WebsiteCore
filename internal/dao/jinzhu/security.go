package jinzhu

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/security"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type securitySrv struct {
	db          *gorm.DB
	codeSecret  string
	phoneVerify core.PhoneVerifyService
	sendEmail   func(context.Context, string, string, time.Duration) error
}

func newSecurityService(db *gorm.DB, phoneVerify core.PhoneVerifyService) core.SecurityService {
	sender := security.NewAliyunMail(conf.AccountVerifySetting.Aliyun)
	return &securitySrv{db: db, codeSecret: conf.JWTSetting.Secret, phoneVerify: phoneVerify, sendEmail: sender.SendCode}
}

var mainlandPhone = regexp.MustCompile(`^(?:\+86)?1[3-9][0-9]{9}$`)
var contactCode = regexp.MustCompile(`^[0-9]{6}$`)

func normalizeContact(mode, address string) (string, error) {
	address = strings.TrimSpace(address)
	if mode == "email" {
		m, err := mail.ParseAddress(address)
		if err != nil || m.Address != address || len(address) > 254 || strings.ContainsAny(address, "\r\n") {
			return "", core.ErrContactInvalid
		}
		return strings.ToLower(address), nil
	}
	if mode != "phone" || !mainlandPhone.MatchString(address) {
		return "", core.ErrContactInvalid
	}
	return strings.TrimPrefix(address, "+86"), nil
}

func (s *securitySrv) contactDigest(parts ...string) string {
	mac := hmac.New(sha256.New, []byte(s.codeSecret))
	for _, part := range parts {
		mac.Write([]byte(part))
		mac.Write([]byte{0})
	}
	return hex.EncodeToString(mac.Sum(nil))
}

func activeContactChannel(mode string) error {
	if mode != conf.VerificationMode() || !conf.VerificationAvailable() {
		return core.ErrContactDisabled
	}
	return nil
}

func (s *securitySrv) SendContactCode(ctx context.Context, userID int64, mode, address, ip string) error {
	if err := activeContactChannel(mode); err != nil {
		return err
	}
	address, err := normalizeContact(mode, address)
	if err != nil || userID <= 0 {
		return core.ErrContactInvalid
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	cfg := conf.AccountVerifySetting
	now := time.Now().Unix()
	record := &dbr.ContactVerification{UserID: userID, Mode: mode, Address: address, IPHash: s.contactDigest("ip", ip), CreatedOn: now, ExpiresOn: now + int64(cfg.CodeTTLSeconds)}
	record.CodeHash = s.contactDigest(fmt.Sprint(userID), mode, address, code)
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		keys := []string{"contact:user:" + fmt.Sprint(userID), "contact:address:" + mode + ":" + address, "contact:ip:" + record.IPHash}
		sort.Strings(keys)
		for _, key := range keys {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error; err != nil {
				return err
			}
		}
		var user dbr.User
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ? AND status = ? AND is_del = 0", userID, dbr.UserStatusNormal).First(&user).Error; err != nil {
			return core.ErrContactInvalid
		}
		var existing int64
		if err := tx.Model(&dbr.User{}).Where(mode+" = ? AND id <> ? AND is_del = 0", address, userID).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return core.ErrContactInUse
		}
		var count int64
		if err := tx.Model(&dbr.ContactVerification{}).Where("created_on > ? AND (user_id = ? OR (mode = ? AND address = ?))", now-int64(cfg.CooldownSeconds), userID, mode, address).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return core.ErrContactRateLimited
		}
		for _, check := range []struct {
			predicate string
			value     any
			limit     int64
		}{
			{"user_id = ?", userID, int64(cfg.MaxDailySends)}, {"address = ?", address, int64(cfg.MaxDailySends)}, {"ip_hash = ?", record.IPHash, int64(cfg.MaxDailySends * 10)},
		} {
			if err := tx.Model(&dbr.ContactVerification{}).Where("created_on > ?", now-86400).Where(check.predicate, check.value).Count(&count).Error; err != nil {
				return err
			}
			if count >= check.limit {
				return core.ErrContactRateLimited
			}
		}
		// Keep bounded per-account verification history without retaining old codes.
		if err := tx.Where("user_id = ? AND created_on < ?", userID, now-7*86400).Delete(&dbr.ContactVerification{}).Error; err != nil {
			return err
		}
		return tx.Create(record).Error
	})
	if err != nil {
		return err
	}
	ttl := time.Duration(cfg.CodeTTLSeconds) * time.Second
	if mode == "email" {
		err = s.sendEmail(ctx, address, code, ttl)
	} else {
		err = s.phoneVerify.SendPhoneCaptcha(address, code, time.Duration(max(int(ttl.Minutes()), 1)))
	}
	if err != nil {
		// Preserve throttling after ambiguous provider failures; never leave their code usable.
		if updateErr := s.db.Model(record).Update("used", true).Error; updateErr != nil {
			return fmt.Errorf("verification delivery and invalidation failed")
		}
		return fmt.Errorf("verification delivery failed")
	}
	return s.db.Model(record).Update("delivered", true).Error
}

func (s *securitySrv) VerifyContactCode(ctx context.Context, userID int64, mode, address, code string) error {
	if err := activeContactChannel(mode); err != nil {
		return err
	}
	address, err := normalizeContact(mode, address)
	if err != nil || !contactCode.MatchString(code) || userID <= 0 {
		return core.ErrContactInvalid
	}
	var result error
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user dbr.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = ? AND is_del = 0", userID, dbr.UserStatusNormal).First(&user).Error; err != nil {
			return core.ErrContactInvalid
		}
		var record dbr.ContactVerification
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).Order("id DESC").First(&record).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return core.ErrContactCode
		}
		if err != nil {
			return err
		}
		if !record.Delivered || record.Used || record.ExpiresOn <= time.Now().Unix() || record.Attempts >= conf.AccountVerifySetting.MaxAttempts {
			return core.ErrContactCode
		}
		record.Attempts++
		matches := record.Mode == mode && record.Address == address && hmac.Equal([]byte(record.CodeHash), []byte(s.contactDigest(fmt.Sprint(userID), mode, address, code)))
		if !matches {
			result = core.ErrContactCode
			return tx.Model(&record).Update("attempts", record.Attempts).Error
		}
		var count int64
		if err := tx.Model(&dbr.User{}).Where(mode+" = ? AND id <> ? AND is_del = 0", address, userID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return core.ErrContactInUse
		}
		if err := tx.Model(&user).Update(mode, address).Error; err != nil {
			return err
		}
		if err := tx.Model(&record).Updates(map[string]any{"used": true, "attempts": record.Attempts}).Error; err != nil {
			return err
		}
		return dbr.AppendOperationLog(tx, userID, "contact.verify", "user", userID, nil, map[string]any{"mode": mode})
	})
	if err != nil {
		var sqlErr interface{ SQLState() string }
		if errors.As(err, &sqlErr) && sqlErr.SQLState() == "23505" {
			return core.ErrContactInUse
		}
		return err
	}
	return result
}
