package acquisition_test

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/acquisition"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestPublicCredentialCompositionDefersSecretReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-secret")
	resolver, err := acquisition.OpenCredentialResolver(t.Context(), acquisition.CredentialResolverConfig{References: []acquisition.CredentialReference{
		{ProviderID: "audit", FieldID: "api-key", Reference: "file:" + path},
	}})
	if err != nil {
		t.Fatal(err)
	}
	provider := publicCredentialProvider()
	if _, err := resolver.ResolveCatalog(t.Context(), &provider); err == nil {
		t.Fatal("missing explicit source produced credentials")
	}
}

func TestPublicCredentialCompositionMigratesAndRetainsPolicy(t *testing.T) {
	t.Setenv("AUDIT_API_KEY", "old-private-material")
	t.Setenv("STARMAP_AUDIT_API_KEY", "new-private-material")
	t.Setenv("CATALOG_SELECTED_KEY", "explicit-private-material")
	path := filepath.Join(t.TempDir(), "policy")
	config := acquisition.CredentialResolverConfig{State: &acquisition.CredentialPolicyState{
		Directory: path, Product: "host", DeploymentID: "team", InstanceID: "one", LegacyInstallation: true,
	}}
	provider := publicCredentialProvider()
	resolver, err := acquisition.OpenCredentialResolver(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.ResolveCatalog(t.Context(), &provider); !errors.IsConflict(err) {
		t.Fatalf("changed legacy selection = %v", err)
	}
	config.References = []acquisition.CredentialReference{{ProviderID: "audit", FieldID: "api-key", Reference: "env:CATALOG_SELECTED_KEY"}}
	resolver, err = acquisition.OpenCredentialResolver(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	material, err := resolver.ResolveCatalog(t.Context(), &provider)
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := material.Value("api-key"); value != "explicit-private-material" {
		t.Fatal("explicit reference did not resolve the migration")
	}
	config.References = nil
	resolver, err = acquisition.OpenCredentialResolver(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	material, err = resolver.ResolveCatalog(t.Context(), &provider)
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := material.Value("api-key"); value != "new-private-material" {
		t.Fatal("accepted policy reverted after restart")
	}
	if err := filepath.WalkDir(path, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if bytes.Contains(data, []byte("private-material")) || bytes.Contains(data, []byte("CATALOG_SELECTED_KEY")) {
			t.Fatal("policy state contains credential material or a source reference")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPublicCredentialCompositionValidatesBeforeStorage(t *testing.T) {
	ref := acquisition.CredentialReference{ProviderID: "audit", FieldID: "api-key", Reference: "env:CATALOG_SELECTED_KEY"}
	for _, references := range [][]acquisition.CredentialReference{{ref, ref}, {{Reference: "env:CATALOG_SELECTED_KEY"}}} {
		path := filepath.Join(t.TempDir(), "policy")
		_, err := acquisition.OpenCredentialResolver(t.Context(), acquisition.CredentialResolverConfig{
			References: references, State: &acquisition.CredentialPolicyState{Directory: path, Product: "host", DeploymentID: "team", InstanceID: "one"},
		})
		if err == nil {
			t.Fatal("invalid references were accepted")
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("invalid configuration created policy storage")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := acquisition.OpenCredentialResolver(ctx, acquisition.CredentialResolverConfig{}); err == nil {
		t.Fatal("canceled construction succeeded")
	}
}

func publicCredentialProvider() catalogs.Provider {
	return catalogs.Provider{ID: "audit", Credentials: &catalogs.ProviderCredentials{
		Fields:             []catalogs.ProviderCredentialField{{ID: "api-key", Kind: catalogs.ProviderCredentialFieldSecret, Required: true, Environment: []string{"AUDIT_API_KEY"}}},
		Profiles:           []catalogs.ProviderCredentialProfile{{ID: "api-key", Primitive: catalogs.ProviderAuthenticationAPIKey, Fields: []catalogs.ProviderCredentialFieldID{"api-key"}}},
		CatalogAcquisition: catalogs.ProviderCredentialPlane{Required: true, Alternatives: []catalogs.ProviderCredentialProfileID{"api-key"}},
	}}
}
