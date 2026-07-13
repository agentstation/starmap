package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestValidatePayloadUsesRegisteredProviderContractWithoutNetwork(t *testing.T) {
	builder, err := catalogs.NewEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	for _, providerID := range []catalogs.ProviderID{catalogs.ProviderIDBaseten, catalogs.ProviderIDXAI} {
		t.Run(providerID.String(), func(t *testing.T) {
			provider, err := builder.Provider(providerID)
			if err != nil {
				t.Fatal(err)
			}
			payload, err := os.ReadFile(filepath.Join("..", "..", "internal", "providers", providerID.String(), "testdata", "models_list.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := validatePayload(context.Background(), provider, payload); err != nil {
				t.Fatalf("validatePayload: %v", err)
			}
		})
	}
}

func TestResponseRevisionPrefersHTTPValidatorsWithoutPayloadDisclosure(t *testing.T) {
	response := &http.Response{Header: http.Header{"Etag": []string{`"revision"`}}}
	revision := responseRevision(response, []byte(`{"secret":"not-returned"}`))
	if revision.Kind != catalogmeta.ObservationRevisionKindETag || revision.Value != `"revision"` {
		t.Fatalf("revision = %#v", revision)
	}
}

func TestConfiguredSecretsExcludesNonSecretRoutingValues(t *testing.T) {
	provider := &catalogs.Provider{
		APIKey: &catalogs.ProviderAPIKey{Name: "PROVIDER_API_KEY"},
		EnvVarValues: map[string]string{
			"PROVIDER_API_KEY": "secret", "PROVIDER_REGION": "fr-par", "PROVIDER_BASE_URL": "https://example.test",
		},
	}
	provider.LoadAPIKey()
	secrets := configuredSecrets(provider)
	if len(secrets) != 1 || string(secrets[0]) != "secret" {
		t.Fatalf("configuredSecrets = %q", secrets)
	}
}
