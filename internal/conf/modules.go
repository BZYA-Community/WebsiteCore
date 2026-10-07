package conf

import (
	"fmt"
	"net/mail"
	"net/url"
)

type CoursesConf struct{ Enabled bool }

type AccountVerifyConf struct {
	Mode            string         `mapstructure:"mode"`
	CodeTTLSeconds  int            `mapstructure:"code_ttl_seconds"`
	CooldownSeconds int            `mapstructure:"cooldown_seconds"`
	MaxAttempts     int            `mapstructure:"max_attempts"`
	MaxDailySends   int            `mapstructure:"max_daily_sends"`
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
	AccountVerifySetting *AccountVerifyConf
)

func CoursesEnabled() bool { return CourseSetting == nil || CourseSetting.Enabled }

func VerificationMode() string {
	if AccountVerifySetting == nil {
		return "email"
	}
	return AccountVerifySetting.Mode
}

func VerificationAvailable() bool {
	if AccountVerifySetting == nil {
		return false
	}
	if VerificationMode() == "phone" {
		return SmsJuheSetting != nil && SmsJuheSetting.Key != "" && SmsJuheSetting.TplID != ""
	}
	a := AccountVerifySetting.Aliyun
	return a.ClientID != "" && a.ClientSecret != "" && a.Sender != ""
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
	return nil
}
