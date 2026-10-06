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
