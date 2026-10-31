package sitesetting

import (
	"context"
	stdjson "encoding/json"
	"reflect"
	"testing"

	"github.com/BZYA-Community/WebsiteCore/internal/conf"
)

func TestTrustedExternalDomainsDefinitionNormalizesValues(t *testing.T) {
	var definition Definition
	for _, candidate := range Registry() {
		if candidate.Key == "web_profile.trusted_external_domains" {
			definition = candidate
			break
		}
	}
	if definition.Key == "" {
		t.Fatal("trusted external domains definition is missing")
	}
	if definition.Type != TypeStringList || definition.ApplyMode != ApplyModeLive || definition.Readonly {
		t.Fatalf("unexpected definition: %+v", definition)
	}

	value, err := definition.ValidateValue([]string{"GitHub.com", "*.docs.example.com", "github.com"})
	if err != nil {
		t.Fatalf("ValidateValue() error = %v", err)
	}
	want := []string{"github.com", "docs.example.com"}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("ValidateValue() = %#v, want %#v", value, want)
	}

	encoded := serializeValue(definition, value)
	decoded, err := parseStoredValue(definition, encoded)
	if err != nil {
		t.Fatalf("parseStoredValue() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("parseStoredValue() = %#v, want %#v", decoded, want)
	}

	if _, err := parseRequestValue(definition, stdjson.RawMessage("null")); err == nil {
		t.Fatal("parseRequestValue(null) error = nil")
	}
	if _, err := parseStoredValue(definition, "null"); err == nil {
		t.Fatal("parseStoredValue(null) error = nil")
	}
}

func TestProfileSerializesEmptyTrustedExternalDomainsAsArray(t *testing.T) {
	previous := conf.WebProfileSetting
	t.Cleanup(func() { conf.WebProfileSetting = previous })
	conf.WebProfileSetting = &conf.WebProfileConf{}
	conf.SetTrustedExternalDomains(nil)

	profile, err := (&Service{}).GetProfile(context.Background())
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	encoded, err := stdjson.Marshal(profile)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var payload map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got := string(payload["trusted_external_domains"]); got != "[]" {
		t.Fatalf("trusted_external_domains = %s, want []", got)
	}
}
