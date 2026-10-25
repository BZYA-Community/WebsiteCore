package conf

import (
	"reflect"
	"testing"
)

func TestNormalizeTrustedExternalDomains(t *testing.T) {
	got, err := NormalizeTrustedExternalDomains([]string{" GitHub.COM. ", "*.Docs.Example.com", ".github.com"})
	if err != nil {
		t.Fatalf("NormalizeTrustedExternalDomains() error = %v", err)
	}
	want := []string{"github.com", "docs.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeTrustedExternalDomains() = %#v, want %#v", got, want)
	}
}

func TestNormalizeTrustedExternalDomainsRejectsURLSyntax(t *testing.T) {
	for _, value := range []string{
		"https://github.com",
		"github.com/path",
		"github.com:443",
		"user@github.com",
		"127.0.0.1",
		"com",
		"co.uk",
		"github.io",
		"",
	} {
		if _, err := NormalizeTrustedExternalDomains([]string{value}); err == nil {
			t.Fatalf("NormalizeTrustedExternalDomains(%q) error = nil", value)
		}
	}
}

func TestTrustedExternalDomainsPreservesEmptyList(t *testing.T) {
	previous := WebProfileSetting
	t.Cleanup(func() { WebProfileSetting = previous })
	WebProfileSetting = &WebProfileConf{}

	SetTrustedExternalDomains(nil)
	if got := TrustedExternalDomains(); got == nil || len(got) != 0 {
		t.Fatalf("TrustedExternalDomains() = %#v, want non-nil empty list", got)
	}
	if WebProfileSetting.TrustedExternalDomains == nil {
		t.Fatal("SetTrustedExternalDomains(nil) stored a nil list")
	}
}
