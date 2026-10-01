// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package core

import (
	"errors"
	"time"
)

var ErrPhoneCaptchaMaxAttempts = errors.New("phone captcha reached maximum attempts")

// SecurityService 安全相关服务
type SecurityService interface {
	VerifyPhoneCaptcha(phone, captcha string, maxAttempts int) (bool, error)
	SendPhoneCaptcha(phone string) error
}

// AttachmentCheckService 附件检测服务
type AttachmentCheckService interface {
	CheckAttachment(uri string) error
}

// PhoneVerifyService 手机验证服务
type PhoneVerifyService interface {
	SendPhoneCaptcha(phone string, captcha string, expire time.Duration) error
}
