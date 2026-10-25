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
