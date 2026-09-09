package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/runtime"
)

func TestApplicationRestartRevokesMetadataSource(t *testing.T) {
	for _, source := range []sources.ID{sources.ModelsDevHTTPID, sources.ModelsDevGitID} {
		t.Run(string(source), func(t *testing.T) { testApplicationSourceRevocation(t, source) })
	}
}

func testApplicationSourceRevocation(t *testing.T, source sources.ID) {
	clearCatalogEnvironment(t)
	t.Setenv("STARMAP_HOME", t.TempDir())
	metadata := newApplicationMetadataFixture(t, source)
	baseline := catalogs.NewEmpty()
	if err := baseline.SetAuthor(catalogs.Author{ID: "acme", Name: "Acme"}); err != nil {
		t.Fatal(err)
	}
	if err := baseline.SetAuthorModel("acme", catalogs.Model{ID: "known", Name: "Known", Authors: []catalogs.Author{{ID: "acme", Name: "Acme"}}}); err != nil {
		t.Fatal(err)
	}
	if err := baseline.SetProvider(catalogs.Provider{ID: "acme", Name: "Acme", Models: map[string]*catalogs.Model{"known": {ID: "known", ModelRef: "acme/known", Name: "Known"}}}); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(t.TempDir(), "workspace")
	if err := baseline.SaveTo(workspace); err != nil {
		t.Fatal(err)
	}
	payload, err := catalogs.EncodeCatalogPayload(baseline)
	if err != nil {
		t.Fatal(err)
	}
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(baselinePath, payload, constants.SecureFilePermissions); err != nil {
		t.Fatal(err)
	}
	open := func(selected string) (*App, *runtime.Runtime) {
		t.Helper()
		values := map[string]string{catalogconfig.Source: "file", catalogconfig.SourceURL: baselinePath, catalogconfig.SourceStartupPolicy: "require_source", catalogconfig.SourcePollInterval: "0s", catalogconfig.AcquisitionEnabled: "false"}
		metadata.configure(values)
		values[catalogconfig.AcquisitionSources] = selected
		application, err := New("test", "test", "test", "test", WithConfig(&Config{Quiet: true, CatalogPath: workspace, CatalogValues: values}))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := application.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		connected, err := application.Runtime(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		return application, connected
	}
	application, connected := open(string(source))
	if _, err := connected.Sync(t.Context()); err != nil {
		t.Fatal(err)
	}
	accepted := connected.State()
	provider, err := accepted.Catalog.Provider("acme")
	if err != nil {
		t.Fatal(err)
	}
	if provider.Models["known"].Description != "Metadata fixture" {
		t.Fatal("selected source did not supply the changed field")
	}
	found := false
	for _, receipt := range connected.Status().SourceObservations {
		if receipt.Source == source {
			found = true
		}
	}
	if !found {
		t.Fatal("accepted source has no receipt")
	}
	calls := metadata.count(t)
	if calls == 0 {
		t.Fatal("selected source did not run")
	}
	if err := application.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		application, connected = open(string(sources.ProvidersID))
		state := connected.State()
		provider, err = state.Catalog.Provider("acme")
		if err != nil {
			t.Fatal("source removal lost the verified baseline", err)
		}
		if provider.Models["known"].Description != "" {
			t.Fatal("disabled source field survived application restart")
		}
		if state.GenerationID == accepted.GenerationID {
			t.Fatal("source removal retained the old generation")
		}
		if connected.Client().CurrentGenerationID() != state.GenerationID {
			t.Fatal("client retained a revoked generation")
		}
		generation, err := connected.Client().Generation(t.Context(), state.GenerationID)
		if err != nil {
			t.Fatal(err)
		}
		for _, receipt := range generation.Manifest.SourceObservations {
			if receipt.Source == source {
				t.Fatal("active manifest retained the revoked source")
			}
		}
		for _, receipt := range connected.Status().SourceObservations {
			if receipt.Source == source {
				t.Fatal("active status retained the revoked source")
			}
		}
		if metadata.count(t) != calls {
			t.Fatal("disabled source ran during application restart")
		}
		if err := application.Shutdown(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}
