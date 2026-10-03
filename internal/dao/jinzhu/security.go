// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jinzhu

import (
	"crypto/rand"
	"math/big"
	"strconv"
	"time"

	"github.com/BZYA-Community/WebsiteCore/internal/core"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
)

type securitySrv struct {
	db          *gorm.DB
	phoneVerify core.PhoneVerifyService
	emailVerify core.EmailVerifyService
}

func newSecurityService(db *gorm.DB, phoneVerify core.PhoneVerifyService, emailVerify core.EmailVerifyService) core.SecurityService {
	return &securitySrv{
		db:          db,
		phoneVerify: phoneVerify,
		emailVerify: emailVerify,
	}
}

// GetLatestPhoneCaptcha 获取最新短信验证码
func (s *securitySrv) GetLatestPhoneCaptcha(phone string) (*ms.Captcha, error) {
	return (&dbr.Captcha{
		Phone: phone,
	}).Get(s.db)
}

// UsePhoneCaptcha 更新短信验证码
func (s *securitySrv) UsePhoneCaptcha(captcha *ms.Captcha) error {
	captcha.UseTimes++
	return captcha.Update(s.db)
}

// SendPhoneCaptcha 发送短信验证码
func (s *securitySrv) SendPhoneCaptcha(phone string) error {
	expire := time.Duration(5)

	// 发送验证码
	captcha, err := generateVerificationCode()
	if err != nil {
		return err
	}
	if err := s.phoneVerify.SendPhoneCaptcha(phone, captcha, expire); err != nil {
		return err
	}

	// 写入表
	captchaModel := &dbr.Captcha{
		Phone:     phone,
		Captcha:   captcha,
		ExpiredOn: time.Now().Add(expire * time.Minute).Unix(),
	}
	captchaModel.Create(s.db)
	return nil
}

func generateVerificationCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(n.Int64()+100000, 10), nil
}

func (s *securitySrv) SendEmailCaptcha(email string) error {
	const expire = 5
	code, err := generateVerificationCode()
	if err != nil {
		return err
	}
	captcha := &dbr.Captcha{Email: email, Captcha: code, ExpiredOn: time.Now().Add(expire * time.Minute).Unix()}
	if _, err = captcha.Create(s.db); err != nil {
		return err
	}
	if err = s.emailVerify.SendEmailCaptcha(email, code, expire); err != nil {
		_ = s.db.Delete(captcha).Error
		return err
	}
	return nil
}

// VerifyEmailCaptcha atomically consumes an attempt. A successful code is
// exhausted immediately so concurrent requests cannot reuse it.
func (s *securitySrv) VerifyEmailCaptcha(email, code string, maxAttempts int) (bool, error) {
	if maxAttempts <= 0 {
		return false, nil
	}
	current, err := (&dbr.Captcha{Email: email}).Get(s.db)
	if err != nil {
		return false, err
	}
	if current.UseTimes >= maxAttempts {
		return false, core.ErrEmailCaptchaMaxAttempts
	}
	base := s.db.Model(&dbr.Captcha{}).Where(
		"id = ? AND is_del = 0 AND expired_on >= ? AND use_times < ?",
		current.ID, time.Now().Unix(), maxAttempts,
	)
	if current.Captcha == code {
		result := base.Where("captcha = ?", code).Update("use_times", maxAttempts)
		if result.Error != nil || result.RowsAffected == 1 {
			return result.RowsAffected == 1, result.Error
		}
	} else {
		result := base.UpdateColumn("use_times", gorm.Expr("use_times + 1"))
		if result.Error != nil || result.RowsAffected == 1 {
			return false, result.Error
		}
	}
	var latest dbr.Captcha
	if err := s.db.Where("id = ? AND is_del = 0", current.ID).First(&latest).Error; err != nil {
		return false, err
	}
	if latest.UseTimes >= maxAttempts {
		return false, core.ErrEmailCaptchaMaxAttempts
	}
	return false, nil
}
