package conf

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestTrustedProxyConfiguration(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader("WebServer:\n  trusted_proxies: [127.0.0.1, '2001:db8::/32']\n")); err != nil {
		t.Fatal(err)
	}
	var config httpServerConf
	if err := v.UnmarshalKey("WebServer", &config); err != nil {
		t.Fatal(err)
	}
	if len(config.TrustedProxies) != 2 || config.ValidateTrustedProxies() != nil {
		t.Fatalf("trusted_proxies YAML was not applied: %+v", config)
	}
	for _, invalid := range []string{"proxy.example.test", "https://127.0.0.1", "127.0.0.1:8080", "192.0.2.0/33", "", "::1%zone"} {
		config.TrustedProxies = []string{invalid}
		if config.ValidateTrustedProxies() == nil {
			t.Fatalf("accepted invalid proxy %q", invalid)
		}
	}
	config.TrustedProxies = nil
	if err := config.ValidateTrustedProxies(); err != nil {
		t.Fatalf("safe empty default rejected: %v", err)
	}
}
