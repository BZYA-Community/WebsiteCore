package sitesetting

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
	"github.com/alimy/tryst/cfg"
)

func TestAliyunSMSSettingsProtectCredentials(t *testing.T) {
	previousSMS, previousBootstrap := conf.SmsAliyunSetting, bootstrapConfig
	t.Cleanup(func() { conf.SmsAliyunSetting, bootstrapConfig = previousSMS, previousBootstrap })
	conf.SmsAliyunSetting, bootstrapConfig = nil, nil
	if err := json.Unmarshal([]byte(`{"AccessKeyID":"synthetic-id","AccessKeySecret":"synthetic-secret","SignName":"测试签名","TemplateCode":"SMS_synthetic"}`), &conf.SmsAliyunSetting); err != nil {
		t.Fatal(err)
	}
	definitions := registryMap()
	for _, field := range []string{"access_key_id", "access_key_secret", "sign_name", "template_code"} {
		key := "sms_aliyun." + field
		def, ok := definitions[key]
		if !ok {
			t.Fatalf("missing SMS setting %s", key)
		}
		secret := field == "access_key_id" || field == "access_key_secret"
		if def.Group != "notifications" || def.Section != "sms_aliyun" || def.Secret != secret || def.Readonly || def.ApplyMode != ApplyModeRestartRequired || activeState(def) != cfg.If("Sms") {
			t.Fatalf("incorrect SMS registration for %s", key)
		}
	}
	for key := range definitions {
		if strings.HasPrefix(key, "sms_juhe.") {
			t.Fatalf("retired SMS provider still registered: %s", key)
		}
	}
	response, err := (&Service{registry: definitions}).GetSchema()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range response.Items {
		if item.Key == "sms_aliyun.access_key_id" || item.Key == "sms_aliyun.access_key_secret" {
			if !item.Secret || !item.BootstrapConfigured || item.BootstrapValue != nil {
				t.Fatalf("SMS credential was exposed or lost its configured state: %s", item.Key)
			}
		}
	}
}
