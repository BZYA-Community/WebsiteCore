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
	"github.com/BZYA-Community/WebsiteCore/internal/dao/jinzhu/dbr"
	"gorm.io/gorm"
)

type securitySrv struct {
	db          *gorm.DB
	phoneVerify core.PhoneVerifyService
}

func newSecurityService(db *gorm.DB, phoneVerify core.PhoneVerifyService) core.SecurityService {
	return &securitySrv{
		db:          db,
		phoneVerify: phoneVerify,
	}
}

// VerifyPhoneCaptcha atomically consumes one attempt on the newest live code.
// A successful code is exhausted immediately, so concurrent requests cannot
// reuse it to bind more than one account. Incorrect codes consume an attempt.
func (s *securitySrv) VerifyPhoneCaptcha(phone, captcha string, maxAttempts int) (bool, error) {
	if maxAttempts <= 0 {
		return false, nil
	}
	current, err := (&dbr.Captcha{Phone: phone}).Get(s.db)
	if err != nil {
		return false, err
	}
	if current.UseTimes >= maxAttempts {
		return false, core.ErrPhoneCaptchaMaxAttempts
	}
	base := s.db.Model(&dbr.Captcha{}).
		Where("id = ? AND is_del = 0 AND expired_on >= ? AND use_times < ?", current.ID, time.Now().Unix(), maxAttempts)
	if current.Captcha == captcha {
		result := base.Where("captcha = ?", captcha).Update("use_times", maxAttempts)
		if result.Error != nil || result.RowsAffected == 1 {
			return result.RowsAffected == 1, result.Error
		}
		return false, s.captchaLimitError(current.ID, maxAttempts)
	}
	result := base.UpdateColumn("use_times", gorm.Expr("use_times + 1"))
	if result.Error != nil || result.RowsAffected == 1 {
		return false, result.Error
	}
	return false, s.captchaLimitError(current.ID, maxAttempts)
}

func (s *securitySrv) captchaLimitError(id int64, maxAttempts int) error {
	var latest dbr.Captcha
	if err := s.db.Where("id = ? AND is_del = 0", id).First(&latest).Error; err != nil {
		return err
	}
	if latest.UseTimes >= maxAttempts {
		return core.ErrPhoneCaptchaMaxAttempts
	}
	return nil
}

func generatePhoneCaptcha() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(n.Int64()+100000, 10), nil
}

// SendPhoneCaptcha 发送短信验证码
func (s *securitySrv) SendPhoneCaptcha(phone string) error {
	expire := time.Duration(5)

	// 发送验证码
	captcha, err := generatePhoneCaptcha()
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
