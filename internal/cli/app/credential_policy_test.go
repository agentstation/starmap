package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/auth"
	"github.com/agentstation/starmap/pkg/catalogs"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestApplicationCredentialPolicyDistinguishesFreshAndLegacyStartup(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "fresh"
		if legacy {
			name = "legacy"
		}
		t.Run(name, func(t *testing.T) {
			clearCatalogEnvironment(t)
			setTestHome(t, t.TempDir())
			t.Setenv("STARMAP_HOME", t.TempDir())
			t.Setenv("OPENAI_API_KEY", "old-material")
			t.Setenv("STARMAP_OPENAI_API_KEY", "new-material")
			makeApp := func() *App {
				t.Helper()
				a, err := New("test", "test", "test", "test", WithConfig(&Config{CatalogValues: map[string]string{
					catalogconfig.Source: "embedded", catalogconfig.SourcePollInterval: "0s", catalogconfig.AcquisitionEnabled: "false",
				}}))
				if err != nil {
					t.Fatal(err)
				}
				return a
			}
			a := makeApp()
			paths, err := a.ResolvedPaths()
			if err != nil {
				t.Fatal(err)
			}
			if legacy {
				if err := os.MkdirAll(paths.Baselines.Path, 0o700); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := a.Runtime(t.Context()); err != nil {
					t.Fatal(err)
				}
				if err := a.closeRuntime(); err != nil {
					t.Fatal(err)
				}
			}
			provider := catalogs.Provider{ID: "openai", Credentials: &catalogs.ProviderCredentials{
				Fields:             []catalogs.ProviderCredentialField{{ID: "api-key", Kind: catalogs.ProviderCredentialFieldSecret, Required: true, Environment: []string{"OPENAI_API_KEY"}}},
				Profiles:           []catalogs.ProviderCredentialProfile{{ID: "api-key", Primitive: catalogs.ProviderAuthenticationAPIKey, Fields: []catalogs.ProviderCredentialFieldID{"api-key"}}},
				CatalogAcquisition: catalogs.ProviderCredentialPlane{Required: true, Alternatives: []catalogs.ProviderCredentialProfileID{"api-key"}},
			}}
			for range 2 {
				a = makeApp()
				resolver, err := a.CredentialResolver()
				if err != nil {
					t.Fatal(err)
				}
				material, err := resolver.ResolveCatalog(t.Context(), &provider)
				if legacy {
					if !errors.IsConflict(err) {
						t.Fatalf("legacy startup error = %v", err)
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					if value, _ := material.Value("api-key"); value != "new-material" {
						t.Fatal("fresh startup selected legacy precedence")
					}
				}
			}
			if legacy {
				t.Setenv("OPENAI_API_KEY", "new-material")
				a = makeApp()
				resolver, err := a.CredentialResolver()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := resolver.ResolveCatalog(t.Context(), &provider); err != nil {
					t.Fatal(err)
				}
			}
			policy, err := auth.OpenFilePolicyStore(t.Context(), paths.CredentialPolicy.Path, auth.PolicyOwner{Product: "starmap", Deployment: paths.DeploymentID, Instance: paths.InstanceID}, auth.EnvironmentPolicyLegacy)
			if err != nil {
				t.Fatal(err)
			}
			selected, err := policy.Policy(t.Context(), "openai")
			if err != nil || selected != auth.EnvironmentPolicyCurrent {
				t.Fatal("restart did not retain the accepted current policy")
			}
			if filepath.Dir(paths.CredentialPolicy.Path) != filepath.Join(paths.Roots["state"].Path, "credentials", paths.DeploymentID) {
				t.Fatal("credential policy escaped its canonical root")
			}
		})
	}
}
