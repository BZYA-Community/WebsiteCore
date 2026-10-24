package conf

import "testing"

func TestProjectsEnabled(t *testing.T) {
	previous := ProjectSetting
	t.Cleanup(func() { ProjectSetting = previous })

	ProjectSetting = nil
	if !ProjectsEnabled() {
		t.Fatal("projects should default to enabled when the section is absent")
	}
	ProjectSetting = &ProjectsConf{Enabled: false}
	if ProjectsEnabled() {
		t.Fatal("projects remained enabled after explicitly disabling them")
	}
	ProjectSetting.Enabled = true
	if !ProjectsEnabled() {
		t.Fatal("projects did not become enabled")
	}
}

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
