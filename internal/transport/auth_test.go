package transport

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestCredentialPlacementValue(t *testing.T) {
	tests := []struct {
		name   string
		scheme catalogs.ProviderCredentialScheme
		want   string
	}{
		{name: "direct", scheme: catalogs.ProviderCredentialSchemeDirect, want: "credential"},
		{name: "bearer", scheme: catalogs.ProviderCredentialSchemeBearer, want: "Bearer credential"},
		{name: "basic", scheme: catalogs.ProviderCredentialSchemeBasic, want: "Basic credential"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := credentialPlacementValue(test.scheme, "credential"); got != test.want {
				t.Fatalf("credentialPlacementValue() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestApplyCredentialMaterialUsesCatalogPlacements(t *testing.T) {
	const secret = " exact-secret-bytes "
	profile := catalogs.ProviderCredentialProfile{
		ID:        "api-key",
		Primitive: catalogs.ProviderAuthenticationAPIKey,
		Fields:    []catalogs.ProviderCredentialFieldID{"api-key"},
		Placements: []catalogs.ProviderCredentialPlacement{
			{
				Field:  "api-key",
				Kind:   catalogs.ProviderCredentialPlacementHeader,
				Name:   "Authorization",
				Scheme: catalogs.ProviderCredentialSchemeBearer,
			},
			{
				Field:  "api-key",
				Kind:   catalogs.ProviderCredentialPlacementQuery,
				Name:   "key",
				Scheme: catalogs.ProviderCredentialSchemeDirect,
			},
		},
	}
	material := sources.NewProviderCredentialMaterial(
		profile,
		map[catalogs.ProviderCredentialFieldID]string{"api-key": secret},
		sources.ProviderCredentialMetadata{Version: "test"},
	)
	req := &http.Request{
		URL:    mustParseURL(t, "https://provider.example/models?existing=value"),
		Header: make(http.Header),
	}

	applyCredentialMaterial(req, material)

	if got := req.Header.Get("Authorization"); got != "Bearer "+secret {
		t.Fatalf("Authorization = %q, want exact catalog transformation", got)
	}
	if got := req.URL.Query().Get("key"); got != secret {
		t.Fatalf("key = %q, want exact secret bytes", got)
	}
	if got := req.URL.Query().Get("existing"); got != "value" {
		t.Fatalf("existing = %q, want preserved query value", got)
	}
}

func TestApplyCredentialMaterialSkipsMissingValues(t *testing.T) {
	profile := catalogs.ProviderCredentialProfile{
		Placements: []catalogs.ProviderCredentialPlacement{{
			Field: "api-key", Kind: catalogs.ProviderCredentialPlacementHeader,
			Name: "x-api-key", Scheme: catalogs.ProviderCredentialSchemeDirect,
		}},
	}
	req := &http.Request{Header: make(http.Header)}

	applyCredentialMaterial(req, sources.NewProviderCredentialMaterial(
		profile,
		nil,
		sources.ProviderCredentialMetadata{Version: "test"},
	))

	if len(req.Header) != 0 {
		t.Fatalf("headers = %v, want no placement for missing value", req.Header)
	}
}

func TestQueryAuthWithNilURLIsNoOp(t *testing.T) {
	req := &http.Request{Header: make(http.Header)}
	QueryAuth{}.Apply(req, catalogs.ProviderCredentialPlacement{
		Kind: catalogs.ProviderCredentialPlacementQuery,
		Name: "key",
	}, "credential")
	if req.URL != nil || len(req.Header) != 0 {
		t.Fatal("query authenticator changed a request without a URL")
	}
}

func mustParseURL(t *testing.T, value string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", value, err)
	}
	return parsed
}
