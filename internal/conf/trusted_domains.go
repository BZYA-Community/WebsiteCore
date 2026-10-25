// Copyright 2026 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package conf

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"golang.org/x/net/publicsuffix"
)

const maxTrustedExternalDomains = 100

var trustedExternalDomainsMu sync.RWMutex

// NormalizeTrustedExternalDomains validates bare hostnames and returns a
// stable, lower-case, de-duplicated list. A rule trusts the hostname itself
// and its subdomains; schemes, paths, ports and userinfo are intentionally
// rejected.
func NormalizeTrustedExternalDomains(values []string) ([]string, error) {
	if len(values) > maxTrustedExternalDomains {
		return nil, fmt.Errorf("trusted_external_domains must contain at most %d domains", maxTrustedExternalDomains)
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		domain := strings.ToLower(strings.TrimSpace(raw))
		domain = strings.TrimPrefix(domain, "*.")
		domain = strings.TrimPrefix(domain, ".")
		domain = strings.TrimRight(domain, ".")
		if err := validateTrustedExternalDomain(domain); err != nil {
			return nil, fmt.Errorf("invalid trusted external domain %q: %w", raw, err)
		}
		if _, exists := seen[domain]; exists {
			continue
		}
		seen[domain] = struct{}{}
		result = append(result, domain)
	}
	return result, nil
}

func validateTrustedExternalDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain must not be empty")
	}
	if len(domain) > 253 {
		return fmt.Errorf("domain is too long")
	}
	if strings.ContainsAny(domain, ":/?#@[]") {
		return fmt.Errorf("use a hostname without scheme, path, port or userinfo")
	}
	if net.ParseIP(domain) != nil {
		return fmt.Errorf("IP addresses cannot be trusted-domain rules")
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" || len(label) > 63 {
			return fmt.Errorf("domain contains an invalid label")
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("domain labels must not start or end with a hyphen")
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return fmt.Errorf("domain must use ASCII letters, digits, dots and hyphens")
			}
		}
	}
	if suffix, _ := publicsuffix.PublicSuffix(domain); suffix == domain {
		return fmt.Errorf("domain must be registrable, not a public suffix")
	}
	return nil
}

func SetTrustedExternalDomains(values []string) {
	trustedExternalDomainsMu.Lock()
	defer trustedExternalDomainsMu.Unlock()
	if WebProfileSetting != nil {
		WebProfileSetting.TrustedExternalDomains = append([]string(nil), values...)
	}
}

func TrustedExternalDomains() []string {
	trustedExternalDomainsMu.RLock()
	defer trustedExternalDomainsMu.RUnlock()
	if WebProfileSetting == nil {
		return nil
	}
	return append([]string(nil), WebProfileSetting.TrustedExternalDomains...)
}
