package sources

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestProviderCredentialMaterialOwnsLifecycleAndRedactsSecrets(t *testing.T) {
	expiresAt := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	refreshAfter := expiresAt.Add(-time.Minute)
	lease := &ProviderCredentialLease{Renewable: true, RefreshAfter: refreshAfter}
	values := map[catalogs.ProviderCredentialFieldID]string{"api-key": "never-render-this-secret"}
	profile := catalogs.ProviderCredentialProfile{
		ID: "api-key", Primitive: catalogs.ProviderAuthenticationAPIKey,
		Fields: []catalogs.ProviderCredentialFieldID{"api-key"},
	}
	material := NewProviderCredentialMaterial(profile, values, ProviderCredentialMetadata{
		Version: "opaque-version", ExpiresAt: expiresAt, Lease: lease,
	})

	values["api-key"] = "mutated"
	profile.Fields[0] = "mutated"
	lease.RefreshAfter = time.Time{}
	if value, _ := material.Value("api-key"); value != "never-render-this-secret" {
		t.Fatalf("material value changed through constructor input: %q", value)
	}
	if got := material.Profile().Fields[0]; got != "api-key" {
		t.Fatalf("material profile changed through constructor input: %q", got)
	}
	gotLease, found := material.Lease()
	if !found || !gotLease.RefreshAfter.Equal(refreshAfter) {
		t.Fatalf("material lease = %#v, %t", gotLease, found)
	}
	gotLease.RefreshAfter = time.Time{}
	secondLease, _ := material.Lease()
	if !secondLease.RefreshAfter.Equal(refreshAfter) {
		t.Fatal("material lease changed through returned value")
	}

	jsonValue, err := json.Marshal(material)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	rendered := []string{fmt.Sprint(material), fmt.Sprintf("%#v", material), string(jsonValue)}
	for _, value := range rendered {
		if strings.Contains(value, "never-render-this-secret") {
			t.Fatalf("generic rendering exposed secret: %s", value)
		}
	}
}

func TestCredentialEndpointBindingsEncodeDeclaredFormats(t *testing.T) {
	profile := catalogs.ProviderCredentialProfile{
		EndpointBindings: []catalogs.ProviderCredentialEndpointBinding{
			{
				Field: "base-url", Variable: "base_url",
				Format: catalogs.ProviderCredentialEndpointBindingURL,
			},
			{
				Field: "project", Variable: "project",
				Format: catalogs.ProviderCredentialEndpointBindingPathSegment,
			},
		},
	}
	material := NewProviderCredentialMaterial(
		profile,
		map[catalogs.ProviderCredentialFieldID]string{
			"base-url": "https://private.example.test/root",
			"project":  "tenant/project?admin=true",
		},
		ProviderCredentialMetadata{Version: "test"},
	)

	bindings := material.EndpointBindings()
	if got := bindings["base_url"]; got != "https://private.example.test/root" {
		t.Fatalf("base_url = %q, want validated URL", got)
	}
	if got := bindings["project"]; got != "tenant%2Fproject%3Fadmin=true" {
		t.Fatalf("project = %q, want one encoded path segment", got)
	}
}

func TestCredentialEndpointBindingsRejectUnsafeURLValues(t *testing.T) {
	profile := catalogs.ProviderCredentialProfile{
		EndpointBindings: []catalogs.ProviderCredentialEndpointBinding{{
			Field: "base-url", Variable: "base_url",
			Format: catalogs.ProviderCredentialEndpointBindingURL,
		}},
	}
	placeholder := strings.Repeat("x", 8)
	for _, value := range []string{
		"https://user:" + placeholder + "@private.example.test",
		"https://private.example.test?api_key=" + placeholder,
		"file:///tmp/provider",
	} {
		t.Run(value, func(t *testing.T) {
			material := NewProviderCredentialMaterial(
				profile,
				map[catalogs.ProviderCredentialFieldID]string{"base-url": value},
				ProviderCredentialMetadata{Version: "test"},
			)
			if bindings := material.EndpointBindings(); len(bindings) != 0 {
				t.Fatalf("unsafe URL produced endpoint bindings: %#v", bindings)
			}
		})
	}
}
