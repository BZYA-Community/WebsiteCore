package conf

import (
	"fmt"
	"net"
)

func (s *httpServerConf) ValidateTrustedProxies() error {
	if s == nil {
		return nil
	}
	for _, proxy := range s.TrustedProxies {
		if net.ParseIP(proxy) != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(proxy); err != nil {
			return fmt.Errorf("WebServer.trusted_proxies must contain only IP addresses or CIDR ranges: %q", proxy)
		}
	}
	return nil
}
