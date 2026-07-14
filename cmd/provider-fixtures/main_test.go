package main

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
)

func TestValidatePayloadUsesRegisteredProviderContractWithoutNetwork(t *testing.T) {
	builder, err := catalogs.NewEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	for _, providerID := range []catalogs.ProviderID{catalogs.ProviderIDMistralAI, catalogs.ProviderIDXAI} {
		t.Run(providerID.String(), func(t *testing.T) {
			provider, err := builder.Provider(providerID)
			if err != nil {
				t.Fatal(err)
			}
			payload, err := os.ReadFile(filepath.Join("..", "..", "internal", "providers", providerID.String(), "testdata", "models_list.json"))
			if err != nil {
				t.Fatal(err)
			}
			if provider.Catalog == nil || len(provider.Catalog.Sources) != 1 {
				t.Fatalf("expected one source, got %#v", provider.Catalog)
			}
			if err := validatePayload(context.Background(), provider, provider.Catalog.Sources[0].ID, payload); err != nil {
				t.Fatalf("validatePayload: %v", err)
			}
		})
	}
}

func TestResponseRevisionPrefersHTTPValidatorsWithoutPayloadDisclosure(t *testing.T) {
	header := http.Header{"Etag": []string{`"revision"`}}
	revision := responseRevision(header, []byte(`{"secret":"not-returned"}`))
	if revision.Kind != catalogmeta.ObservationRevisionKindETag || revision.Value != `"revision"` {
		t.Fatalf("revision = %#v", revision)
	}
}

func TestConfiguredSecretsExcludesNonSecretRoutingValues(t *testing.T) {
	provider := &catalogs.Provider{
		Credentials: map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{"api_key": {Env: catalogs.ProviderEnvironmentNames{"PROVIDER_API_KEY"}}},
	}
	t.Setenv("PROVIDER_API_KEY", "secret")
	t.Setenv("PROVIDER_REGION", "fr-par")
	t.Setenv("PROVIDER_BASE_URL", "https://example.test")
	secrets := configuredSecrets(provider)
	if len(secrets) != 1 || string(secrets[0]) != "secret" {
		t.Fatalf("configuredSecrets = %q", secrets)
	}
}

func TestProviderFixtureRefreshCommandFailurePropagatesNonZero(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("repository root: %v", err)
	}
	fakeGo := filepath.Join(t.TempDir(), "fake-go")
	if err := os.WriteFile(fakeGo, []byte("#!/bin/sh\nexit 42\n"), constants.ExecutablePermissions); err != nil {
		t.Fatalf("WriteFile fake go: %v", err)
	}
	command := exec.Command("bash", filepath.Join(root, "scripts", "refresh-provider-testdata.sh"), "openai")
	command.Dir = root
	command.Env = append(os.Environ(), "STARMAP_GO_RUN_BIN="+fakeGo)
	if err := command.Run(); err == nil {
		t.Fatal("provider refresh helper suppressed command failure")
	}
}
