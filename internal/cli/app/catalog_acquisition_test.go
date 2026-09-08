package app

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestCatalogAcquisitionUsesConfiguredBindings(t *testing.T) {
	clearCatalogEnvironment(t)
	t.Setenv("STARMAP_HOME", t.TempDir())
	client, err := starmap.New()
	if err != nil {
		t.Fatal(err)
	}
	provider, err := client.Catalog().Provider("openai")
	if err != nil {
		t.Fatal(err)
	}
	if provider.Credentials == nil || len(provider.Credentials.Profiles) == 0 {
		t.Fatal("OpenAI fixture has no credential profile")
	}
	first := sources.ProviderAcquisitionBinding{
		SchemaVersion: 1, ID: "first", Revision: "1", ProviderID: provider.ID,
		AccountID: "first-account", Region: "global", APISurface: "models.list",
		CredentialRole:      sources.ProviderBindingCatalogAcquisition,
		CredentialProfileID: provider.Credentials.Profiles[0].ID,
	}
	second := first
	second.ID, second.AccountID = "second", "second-account"
	encoded, err := json.Marshal([]sources.ProviderAcquisitionBinding{first, second})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		values  map[string]string
		want    int32
		blocked bool
	}{
		{name: "omitted", want: 1},
		{name: "explicit-empty", values: map[string]string{catalogconfig.ProviderBindings: "[]"}, blocked: true},
		{name: "two-scopes", values: map[string]string{catalogconfig.ProviderBindings: string(encoded)}, want: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			settings, err := catalogconfig.Parse(test.values)
			if err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int32
			application := &App{config: &Config{}, catalogSettings: settings,
				credentialResolver: sources.ProviderCredentialResolverFunc(func(context.Context, *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
					calls.Add(1)
					return sources.ProviderCredentialMaterial{}, &errors.ValidationError{Field: "test credentials", Message: "unavailable"}
				}),
			}
			t.Cleanup(func() { _ = application.Shutdown(context.Background()) })
			syncer, err := application.CatalogAcquisition(client)
			if err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 0 {
				t.Fatal("construction resolved acquisition credentials")
			}
			before := client.Catalog()
			_, err = syncer.Sync(t.Context(), pkgsync.WithDryRun(true), pkgsync.WithSources(sources.ProvidersID), pkgsync.WithProvider(provider.ID))
			if (err != nil) != test.blocked {
				t.Fatalf("requested provider error = %v, want blocked %t", err, test.blocked)
			}
			if test.blocked {
				if _, err := syncer.Sync(t.Context(), pkgsync.WithDryRun(true), pkgsync.WithSources(sources.ProvidersID)); err != nil {
					t.Fatalf("empty unrestricted acquisition: %v", err)
				}
			}
			if calls.Load() != test.want {
				t.Fatalf("credential acquisitions = %d, want %d", calls.Load(), test.want)
			}
			if client.Catalog() != before {
				t.Fatal("preview replaced the current catalog")
			}
		})
	}
}
