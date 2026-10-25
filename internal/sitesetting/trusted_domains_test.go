package sitesetting

import (
	stdjson "encoding/json"
	"reflect"
	"testing"
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
