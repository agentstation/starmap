package catalogs

import "testing"

func TestProviderCatalogEndpointURLUsesConfiguredURL(t *testing.T) {
	provider := &Provider{Catalog: &ProviderCatalog{Endpoint: ProviderEndpoint{
		URL: "https://api.example.test/v1/models",
	}}}

	if got := provider.CatalogEndpointURL(); got != "https://api.example.test/v1/models" {
		t.Fatalf("CatalogEndpointURL() = %q", got)
	}
}

func TestProviderBindCatalogEndpointUsesResolvedParameters(t *testing.T) {
	provider := &Provider{Catalog: &ProviderCatalog{Endpoint: ProviderEndpoint{
		URL: "{base_url}/v1/projects/{project}/models",
	}}}

	got, err := provider.BindCatalogEndpoint(map[string]string{
		"base_url": "https://private.example.test",
		"project":  "tenant-project",
	})
	if err != nil {
		t.Fatalf("BindCatalogEndpoint: %v", err)
	}
	if want := "https://private.example.test/v1/projects/tenant-project/models"; got != want {
		t.Fatalf("BindCatalogEndpoint() = %q, want %q", got, want)
	}
}

func TestProviderBindCatalogEndpointFailsClosed(t *testing.T) {
	provider := &Provider{Catalog: &ProviderCatalog{Endpoint: ProviderEndpoint{
		URL: "https://api.example.test/projects/{project}/models",
	}}}

	if _, err := provider.BindCatalogEndpoint(nil); err == nil {
		t.Fatal("BindCatalogEndpoint accepted an unresolved variable")
	}
	if _, err := provider.BindCatalogEndpoint(map[string]string{
		"project": "value@evil.example/path",
	}); err != nil {
		t.Fatalf("path parameter changed endpoint authority: %v", err)
	}
}
