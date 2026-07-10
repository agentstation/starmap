package catalogs

import "testing"

func TestProviderCatalogEndpointURLUsesConfiguredURL(t *testing.T) {
	provider := &Provider{
		Catalog: &ProviderCatalog{
			Endpoint: ProviderEndpoint{
				URL: "https://dashscope-us.aliyuncs.com/compatible-mode/v1/models",
			},
		},
	}

	got := provider.CatalogEndpointURL()
	want := "https://dashscope-us.aliyuncs.com/compatible-mode/v1/models"
	if got != want {
		t.Fatalf("CatalogEndpointURL() = %q, want %q", got, want)
	}
}

func TestProviderCatalogEndpointURLUsesBaseURLEnvVar(t *testing.T) {
	provider := &Provider{
		Catalog: &ProviderCatalog{
			Endpoint: ProviderEndpoint{
				URL:           "https://dashscope-us.aliyuncs.com/compatible-mode/v1/models",
				BaseURLEnvVar: "ALIBABA_MODEL_STUDIO_BASE_URL",
				Path:          "/models",
			},
		},
	}
	t.Setenv("ALIBABA_MODEL_STUDIO_BASE_URL", "https://example.ap-southeast-1.maas.aliyuncs.com/compatible-mode/v1/")

	got := provider.CatalogEndpointURL()
	want := "https://example.ap-southeast-1.maas.aliyuncs.com/compatible-mode/v1/models"
	if got != want {
		t.Fatalf("CatalogEndpointURL() = %q, want %q", got, want)
	}
}

func TestProviderCatalogEndpointURLUsesLoadedEnvVarValue(t *testing.T) {
	provider := &Provider{
		EnvVarValues: map[string]string{
			"ALIBABA_MODEL_STUDIO_BASE_URL": "https://workspace.cn-beijing.maas.aliyuncs.com/compatible-mode/v1",
		},
		Catalog: &ProviderCatalog{
			Endpoint: ProviderEndpoint{
				URL:           "https://dashscope-us.aliyuncs.com/compatible-mode/v1/models",
				BaseURLEnvVar: "ALIBABA_MODEL_STUDIO_BASE_URL",
				Path:          "models",
			},
		},
	}

	got := provider.CatalogEndpointURL()
	want := "https://workspace.cn-beijing.maas.aliyuncs.com/compatible-mode/v1/models"
	if got != want {
		t.Fatalf("CatalogEndpointURL() = %q, want %q", got, want)
	}
}
