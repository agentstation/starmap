package providers_test

import (
	"os"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestExactOpenAIProvidersUseTableDrivenConfigurationComposition(t *testing.T) {
	builder, err := catalogs.NewFromFS(os.DirFS("../embedded/catalog"), ".")
	if err != nil {
		t.Fatalf("NewFromFS: %v", err)
	}
	tests := []struct {
		id            catalogs.ProviderID
		endpoint      string
		authorRules   int
		offeringTier  string
		regionalScope bool
	}{
		{catalogs.ProviderIDAlibabaCloud, "https://dashscope-us.aliyuncs.com/compatible-mode/v1/models", 8, "", false},
		{catalogs.ProviderIDBaseten, "https://inference.baseten.co/v1/models", 5, "model-api", false},
		{catalogs.ProviderIDDeepSeek, "https://api.deepseek.com/models", 0, "", false},
		{catalogs.ProviderIDHyperbolic, "https://api.hyperbolic.xyz/v1/models", 5, "pay-per-token", false},
		{catalogs.ProviderIDMoonshotAI, "https://api.moonshot.ai/v1/models", 1, "", false},
		{catalogs.ProviderIDScaleway, "https://api.scaleway.ai/v1/models", 12, "pay-per-use", true},
	}
	for _, test := range tests {
		t.Run(test.id.String(), func(t *testing.T) {
			provider, err := builder.Provider(test.id)
			if err != nil {
				t.Fatalf("Provider: %v", err)
			}
			if provider.Catalog == nil || len(provider.Catalog.Sources) != 1 {
				t.Fatalf("sources = %#v", provider.Catalog)
			}
			source := provider.Catalog.Sources[0]
			if source.Endpoint.Type != catalogs.EndpointTypeOpenAI || source.Endpoint.URL != test.endpoint {
				t.Fatalf("endpoint = %#v, want %q", source.Endpoint, test.endpoint)
			}
			if source.Endpoint.ResponseCollection != "" || len(source.Endpoint.FieldMappings) != 0 || len(source.Endpoint.FeatureRules) != 0 {
				t.Fatalf("exact OpenAI source has wire-schema delta: %#v", source.Endpoint)
			}
			rules := 0
			if source.Endpoint.AuthorMapping != nil {
				rules = len(source.Endpoint.AuthorMapping.Normalized)
			}
			if rules != test.authorRules {
				t.Fatalf("author rules = %d, want %d", rules, test.authorRules)
			}
			tier := ""
			if source.Offering != nil {
				tier = source.Offering.Deployment.Tier
			}
			if tier != test.offeringTier {
				t.Fatalf("offering tier = %q, want %q", tier, test.offeringTier)
			}
			if (source.ObservationScope.Invariant == catalogs.ProviderObservationScopeRegionalPublic) != test.regionalScope {
				t.Fatalf("observation scope = %#v", source.ObservationScope)
			}
		})
	}
}

func TestProviderConfigurationSourceParsesStrictly(t *testing.T) {
	builder, err := catalogs.NewFromFS(os.DirFS("../embedded/catalog"), ".")
	if err != nil {
		t.Fatalf("NewFromFS: %v", err)
	}
	for _, id := range []catalogs.ProviderID{
		catalogs.ProviderIDAmazonBedrock,
		catalogs.ProviderIDMicrosoftFoundry,
		catalogs.ProviderIDOCI,
	} {
		provider, err := builder.Provider(id)
		if err != nil {
			t.Fatalf("Provider(%s): %v", id, err)
		}
		if provider.Catalog == nil || len(provider.Catalog.Sources) != 1 || provider.Catalog.Sources[0].ObservationScope.Invariant != catalogs.ProviderObservationScopeCredentialScoped {
			t.Fatalf("native source %s is not one configured credential-scoped source: %#v", id, provider.Catalog)
		}
	}
}

func TestNVIDIAPublicAndNIMSourcesOwnDistinctOfferingPolicy(t *testing.T) {
	builder, err := catalogs.NewFromFS(os.DirFS("../embedded/catalog"), ".")
	if err != nil {
		t.Fatalf("NewFromFS: %v", err)
	}
	provider, err := builder.Provider(catalogs.ProviderIDNVIDIA)
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	if len(provider.Catalog.Sources) != 2 {
		t.Fatalf("NVIDIA sources = %#v", provider.Catalog.Sources)
	}
	public, nim := provider.Catalog.Sources[0], provider.Catalog.Sources[1]
	if public.ID != "public-models" || public.ObservationScope.Invariant != catalogs.ProviderObservationScopeGlobalPublic ||
		public.Offering.Access.Routability != catalogs.OfferingRoutabilityDiscoverable || public.Offering.Deployment.Type != "nvidia-hosted" {
		t.Fatalf("NVIDIA public source = %#v", public)
	}
	if nim.ID != "account-nim-models" || nim.ObservationScope.Invariant != catalogs.ProviderObservationScopeCredentialScoped ||
		nim.Offering.Access.Routability != catalogs.OfferingRoutabilityRoutable || nim.Offering.Deployment.Type != "nim" ||
		nim.Endpoint.BaseURLEnv != "NVIDIA_NIM_BASE_URL" {
		t.Fatalf("NVIDIA NIM source = %#v", nim)
	}
}
