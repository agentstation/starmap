package sources

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

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
	for _, value := range []string{
		"https://user:secret@private.example.test",
		"https://private.example.test?api_key=secret",
		"file:///tmp/provider",
	} {
		t.Run(value, func(t *testing.T) {
			material := NewProviderCredentialMaterial(
				profile,
				map[catalogs.ProviderCredentialFieldID]string{"base-url": value},
			)
			if bindings := material.EndpointBindings(); len(bindings) != 0 {
				t.Fatalf("unsafe URL produced endpoint bindings: %#v", bindings)
			}
		})
	}
}
