package conf

import (
	"fmt"
	"net/mail"
	"net/url"
	"strings"
)

type CoursesConf struct{ Enabled bool }
type ProjectsConf struct{ Enabled bool }

type AccountVerifyConf struct {
	Mode            string         `mapstructure:"mode"`
	CodeTTLSeconds  int            `mapstructure:"code_ttl_seconds"`
	CooldownSeconds int            `mapstructure:"cooldown_seconds"`
	MaxAttempts     int            `mapstructure:"max_attempts"`
	MaxDailySends   int            `mapstructure:"max_daily_sends"`
	DevSkipCode     bool           `mapstructure:"dev_skip_code"`
	Aliyun          AliyunMailConf `mapstructure:"aliyun"`
}

type AliyunMailConf struct {
	Endpoint     string `mapstructure:"endpoint"`
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	Sender       string `mapstructure:"sender"`
}

var (
	CourseSetting        *CoursesConf
	ProjectSetting       *ProjectsConf
	AccountVerifySetting *AccountVerifyConf
)

func CoursesEnabled() bool  { return CourseSetting == nil || CourseSetting.Enabled }
func ProjectsEnabled() bool { return ProjectSetting == nil || ProjectSetting.Enabled }

func VerificationMode() string {
	if AccountVerifySetting == nil {
		return "email"
	}
	return AccountVerifySetting.Mode
}

func VerificationAvailable() bool {
	if ContactVerificationDevBypass() {
		return true
	}
	if AccountVerifySetting == nil {
		return false
	}
	if VerificationMode() == "phone" {
		return SmsConfigured()
	}
	a := AccountVerifySetting.Aliyun
	return a.ClientID != "" && a.ClientSecret != "" && a.Sender != ""
}

func SmsConfigured() bool {
	return SmsAliyunSetting != nil &&
		strings.TrimSpace(SmsAliyunSetting.AccessKeyID) != "" &&
		strings.TrimSpace(SmsAliyunSetting.AccessKeySecret) != "" &&
		strings.TrimSpace(SmsAliyunSetting.SignName) != "" &&
		strings.TrimSpace(SmsAliyunSetting.TemplateCode) != ""
}

// ContactVerificationDevBypass reports whether the development-only contact
// verification bypass is active. When active, sending a code performs no
// provider delivery and verifying accepts any 6-digit code, so local smoke
// tests can use throwaway email addresses or phone numbers.
//
// It requires both the opt-in account_verify.dev_skip_code flag and a
// development run mode; Validate() refuses the flag everywhere else, and this
// second check keeps a misconfigured non-development deployment inert.
func ContactVerificationDevBypass() bool {
	if AccountVerifySetting == nil || !AccountVerifySetting.DevSkipCode {
		return false
	}
	return developmentRunMode()
}

func (c *AccountVerifyConf) Validate() error {
	if c.Mode != "email" && c.Mode != "phone" {
		return fmt.Errorf("account_verify.mode must be email or phone")
	}
	if c.CodeTTLSeconds < 60 || c.CodeTTLSeconds > 1800 || c.CooldownSeconds < 30 || c.CooldownSeconds > 3600 || c.MaxAttempts < 1 || c.MaxAttempts > 10 || c.MaxDailySends < 1 || c.MaxDailySends > 100 {
		return fmt.Errorf("account_verify limits are out of range")
	}
	u, err := url.Parse(c.Aliyun.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return fmt.Errorf("account_verify.aliyun.endpoint must be an HTTPS origin")
	}
	if c.Aliyun.Sender != "" {
		m, err := mail.ParseAddress(c.Aliyun.Sender)
		if err != nil || m.Address != c.Aliyun.Sender {
			return fmt.Errorf("account_verify.aliyun.sender must be an email address")
		}
	}
	if c.DevSkipCode && !developmentRunMode() {
		return fmt.Errorf("account_verify.dev_skip_code requires App.RunMode debug (development only)")
	}
	return nil
}

func developmentRunMode() bool {
	return AppSetting != nil && AppSetting.RunMode == "debug"
}
