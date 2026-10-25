package conf

import "testing"

func TestAccountVerificationConfiguration(t *testing.T) {
	c := AccountVerifyConf{Mode: "email", CodeTTLSeconds: 300, CooldownSeconds: 60, MaxAttempts: 5, MaxDailySends: 10, Aliyun: AliyunMailConf{Endpoint: "https://alimail-cn.aliyuncs.com"}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"", "both", "EMAIL"} {
		c.Mode = mode
		if c.Validate() == nil {
			t.Fatalf("accepted mode %q", mode)
		}
	}
	c.Mode = "phone"
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Aliyun.Endpoint = "http://mail.example.test"
	if c.Validate() == nil {
		t.Fatal("accepted plaintext credential endpoint")
	}
	c.Aliyun.Endpoint = "https://mail.example.test"
	c.MaxAttempts = 0
	if c.Validate() == nil {
		t.Fatal("accepted unbounded/zero attempts")
	}
}

func TestAliyunSMSConfigurationAvailability(t *testing.T) {
	previousSMS, previousVerify, previousApp := SmsAliyunSetting, AccountVerifySetting, AppSetting
	t.Cleanup(func() { SmsAliyunSetting, AccountVerifySetting, AppSetting = previousSMS, previousVerify, previousApp })
	AccountVerifySetting, AppSetting = &AccountVerifyConf{Mode: "phone"}, &appConf{RunMode: "release"}
	SmsAliyunSetting = nil
	if SmsConfigured() || VerificationAvailable() {
		t.Fatal("missing SMS configuration enabled phone verification")
	}
	complete := smsAliyunConf{AccessKeyID: "synthetic-id", AccessKeySecret: "synthetic-secret", SignName: "测试签名", TemplateCode: "SMS_synthetic"}
	SmsAliyunSetting = &complete
	if !SmsConfigured() || !VerificationAvailable() {
		t.Fatal("complete Aliyun SMS configuration did not enable phone verification")
	}
	for _, missing := range []string{"AccessKeyID", "AccessKeySecret", "SignName", "TemplateCode"} {
		partial := complete
		switch missing {
		case "AccessKeyID":
			partial.AccessKeyID = " \t"
		case "AccessKeySecret":
			partial.AccessKeySecret = " \t"
		case "SignName":
			partial.SignName = " \t"
		case "TemplateCode":
			partial.TemplateCode = " \t"
		}
		SmsAliyunSetting = &partial
		if SmsConfigured() || VerificationAvailable() {
			t.Fatalf("blank %s enabled phone verification", missing)
		}
	}
	AccountVerifySetting.DevSkipCode, AppSetting.RunMode = true, "debug"
	if SmsConfigured() || !VerificationAvailable() {
		t.Fatal("explicit development bypass changed the provider configuration requirement")
	}
}

func TestAccountVerificationDevSkipCode(t *testing.T) {
	previousVerify, previousApp := AccountVerifySetting, AppSetting
	t.Cleanup(func() { AccountVerifySetting, AppSetting = previousVerify, previousApp })

	newConf := func(devSkip bool) AccountVerifyConf {
		return AccountVerifyConf{Mode: "email", CodeTTLSeconds: 300, CooldownSeconds: 60, MaxAttempts: 5, MaxDailySends: 10, DevSkipCode: devSkip, Aliyun: AliyunMailConf{Endpoint: "https://alimail-cn.aliyuncs.com"}}
	}

	// Flag off: unchanged behavior, no provider credentials -> unavailable.
	off := newConf(false)
	if err := off.Validate(); err != nil {
		t.Fatal(err)
	}
	AccountVerifySetting, AppSetting = &off, nil
	if ContactVerificationDevBypass() || VerificationAvailable() {
		t.Fatal("flag off activated the development bypass")
	}

	// Flag on outside a development run mode: startup must refuse it.
	on := newConf(true)
	AppSetting = &appConf{RunMode: "release"}
	if err := on.Validate(); err == nil {
		t.Fatal("accepted dev_skip_code with a non-development run mode")
	}
	AccountVerifySetting = &on
	if ContactVerificationDevBypass() || VerificationAvailable() {
		t.Fatal("bypass active outside development")
	}

	// Flag on in debug: verification works without provider credentials.
	AppSetting = &appConf{RunMode: "debug"}
	if err := on.Validate(); err != nil {
		t.Fatal(err)
	}
	if !ContactVerificationDevBypass() || !VerificationAvailable() {
		t.Fatal("debug run mode with dev_skip_code did not enable verification")
	}

	// Missing run mode: inert, the bypass never activates.
	AppSetting = nil
	if ContactVerificationDevBypass() || VerificationAvailable() {
		t.Fatal("bypass active without a run mode")
	}
}
