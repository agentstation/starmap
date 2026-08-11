package auth

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExplicitCredentialReferencesPrecedeAmbientSources(t *testing.T) {
	provider := ambientCredentialProvider()
	key := CredentialFieldKey{ProviderID: provider.ID, FieldID: "api-key"}

	t.Run("explicit environment reference precedes ambient discovery", func(t *testing.T) {
		lookups := make([]string, 0, 2)
		resolver := newResolver(func(name string) (string, bool) {
			lookups = append(lookups, name)
			values := map[string]string{
				"EXPLICIT_OPENAI_KEY": "valid-explicit",
				"OPENAI_API_KEY":      "valid-ambient",
			}
			value, found := values[name]
			return value, found
		}, WithReferencePolicies(map[CredentialFieldKey]ReferencePolicy{
			key: {Reference: mustReference(t, "env:EXPLICIT_OPENAI_KEY")},
		}))
		material, err := resolver.ResolveCatalog(context.Background(), &provider)
		if err != nil {
			t.Fatalf("ResolveCatalog: %v", err)
		}
		if value, _ := material.Value("api-key"); value != "valid-explicit" {
			t.Fatalf("api-key = %q, want explicit value", value)
		}
		if len(lookups) != 1 || lookups[0] != "EXPLICIT_OPENAI_KEY" {
			t.Fatalf("lookups = %#v, want explicit reference only", lookups)
		}
	})

	t.Run("missing explicit reference fails closed", func(t *testing.T) {
		lookups := make([]string, 0, 2)
		resolver := newResolver(func(name string) (string, bool) {
			lookups = append(lookups, name)
			if name == "OPENAI_API_KEY" {
				return "valid-ambient", true
			}
			return "", false
		}, WithReferencePolicies(map[CredentialFieldKey]ReferencePolicy{
			key: {Reference: mustReference(t, "env:MISSING_OPENAI_KEY")},
		}))
		_, err := resolver.ResolveCatalog(context.Background(), &provider)
		if err == nil {
			t.Fatal("ResolveCatalog succeeded with a missing explicit reference")
		}
		if strings.Contains(err.Error(), "MISSING_OPENAI_KEY") ||
			strings.Contains(err.Error(), "valid-ambient") {
			t.Fatalf("error exposed credential source details: %v", err)
		}
		if len(lookups) != 1 || lookups[0] != "MISSING_OPENAI_KEY" {
			t.Fatalf("lookups = %#v, want no ambient fallback", lookups)
		}
	})

	t.Run("configured not-configured fallback permits ambient discovery", func(t *testing.T) {
		resolver := newResolver(mapEnvironment(map[string]string{
			"OPENAI_API_KEY": "valid-ambient",
		}), WithReferencePolicies(map[CredentialFieldKey]ReferencePolicy{
			key: {
				Reference:       mustReference(t, "env:MISSING_OPENAI_KEY"),
				FallbackAmbient: true,
			},
		}))
		material, err := resolver.ResolveCatalog(context.Background(), &provider)
		if err != nil {
			t.Fatalf("ResolveCatalog: %v", err)
		}
		if value, _ := material.Value("api-key"); value != "valid-ambient" {
			t.Fatalf("api-key = %q, want configured ambient fallback", value)
		}
	})

	t.Run("file reference preserves exact bytes", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "provider.key")
		const value = "valid-file-key\n"
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		provider.Credentials.Fields[0].Pattern = ""
		resolver := newResolver(mapEnvironment(nil), WithReferencePolicies(map[CredentialFieldKey]ReferencePolicy{
			key: {Reference: mustReference(t, "file:"+path)},
		}))
		material, err := resolver.ResolveCatalog(context.Background(), &provider)
		if err != nil {
			t.Fatalf("ResolveCatalog: %v", err)
		}
		if got, _ := material.Value("api-key"); got != value {
			t.Fatalf("api-key bytes = %q, want %q", got, value)
		}
	})
}

func TestParseReferenceGrammar(t *testing.T) {
	valid := []struct {
		value    string
		backend  ReferenceBackend
		resource string
		version  string
		field    string
	}{
		{value: "env:OPENAI_API_KEY", backend: "env", resource: "OPENAI_API_KEY"},
		{value: "file:/run/secrets/key", backend: "file", resource: "/run/secrets/key"},
		{
			value: "vault:secret/data/app?version=2#api-key", backend: "vault",
			resource: "secret/data/app", version: "2", field: "api-key",
		},
	}
	for _, test := range valid {
		t.Run(test.value, func(t *testing.T) {
			reference, err := ParseReference(test.value)
			if err != nil {
				t.Fatalf("ParseReference: %v", err)
			}
			if reference.backend != test.backend || reference.resource != test.resource ||
				reference.version != test.version || reference.field != test.field {
				t.Fatalf("reference = %#v", reference)
			}
		})
	}

	for _, value := range []string{
		"", "ENV:name", "env:", "env:NAME?", "env:NAME#",
		"env:NAME?other=x", "env:NAME?version=", "env:NAME?version=1&version=2",
		"env:NAME#field#more",
	} {
		t.Run("invalid "+value, func(t *testing.T) {
			if _, err := ParseReference(value); err == nil {
				t.Fatal("ParseReference accepted invalid reference")
			}
		})
	}
}

func mustReference(t testing.TB, value string) Reference {
	t.Helper()
	reference, err := ParseReference(value)
	if err != nil {
		t.Fatalf("ParseReference(%q): %v", value, err)
	}
	return reference
}
