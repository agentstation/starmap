package auth

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestCloudCredentialChainSelection(t *testing.T) {
	t.Parallel()

	called := false
	checker := NewChecker(WithCredentialResolver(
		catalogs.ProviderCatalogAuthAzureDefault,
		CredentialResolverFunc(func(provider *catalogs.Provider) *Status {
			called = true
			return &Status{
				State:   StateConfigured,
				Summary: "Azure default credential chain configured",
				CredentialChain: &CredentialChainDetails{
					Method: catalogs.ProviderCatalogAuthAzureDefault,
					Source: "workload-identity",
				},
			}
		}),
	))
	provider := &catalogs.Provider{
		ID:   "azure-model-catalog",
		Name: "Azure model catalog",
		Catalog: &catalogs.ProviderCatalog{
			Auth: catalogs.ProviderCatalogAuth{
				Method:   catalogs.ProviderCatalogAuthAzureDefault,
				Required: true,
			},
			Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeOpenAI},
		},
	}

	status := checker.CheckProvider(provider, map[string]bool{string(provider.ID): true})
	if !called {
		t.Fatal("catalog auth metadata did not select the Azure credential resolver")
	}
	if status.State != StateConfigured || status.CredentialChain == nil ||
		status.CredentialChain.Method != catalogs.ProviderCatalogAuthAzureDefault {
		t.Fatalf("credential status = %#v", status)
	}

	called = false
	provider.Catalog.Endpoint.Type = catalogs.EndpointTypeGoogleCloud
	status = checker.CheckProvider(provider, map[string]bool{string(provider.ID): true})
	if !called || status.State != StateConfigured {
		t.Fatalf("endpoint type changed resolver selection: called=%t status=%#v", called, status)
	}
}
