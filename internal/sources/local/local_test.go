package local

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestSourceWithoutWorkspaceDoesNotMasqueradeAsEmbedded(t *testing.T) {
	_, err := New().Observe(context.Background())
	var configError *pkgerrors.ConfigError
	if !stderrors.As(err, &configError) {
		t.Fatalf("Observe error = %T %v, want *errors.ConfigError", err, err)
	}
}

func TestSourcePublishesProvidedSnapshot(t *testing.T) {
	builder := catalogs.NewEmpty()
	snapshot, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	source := New(WithCatalog(snapshot))
	observation, err := source.Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}

	published := observation.Catalog
	if err := observation.Validate(); err != nil {
		t.Fatalf("Validate observation: %v", err)
	}
	if _, ok := any(published).(*catalogs.Builder); ok {
		t.Fatal("Local source exposed the provided mutable builder")
	}
	if err := builder.SetProvider(catalogs.Provider{ID: "later", Name: "Later"}); err != nil {
		t.Fatalf("Mutate builder: %v", err)
	}
	if _, found := published.Providers().Get("later"); found {
		t.Fatal("Published local snapshot observed later builder mutation")
	}
}

func TestSourceObserveIsConcurrentAndRepeatable(t *testing.T) {
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "stable", Name: "Stable"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	source := New(WithCatalog(catalog))

	const calls = 16
	observations := make([]*catalogs.Catalog, calls)
	errs := make([]error, calls)
	var wg sync.WaitGroup
	for i := range calls {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			observation, observeErr := source.Observe(context.Background())
			observations[index] = observation.Catalog
			errs[index] = observeErr
		}(i)
	}
	wg.Wait()

	for i := range calls {
		if errs[i] != nil {
			t.Fatalf("Observe %d: %v", i, errs[i])
		}
		if observations[i] == nil {
			t.Fatalf("Observe %d returned a nil catalog", i)
		}
		provider, providerErr := observations[i].Provider("stable")
		if providerErr != nil {
			t.Fatalf("Observe %d provider: %v", i, providerErr)
		}
		provider.Name = "caller mutation"
	}

	provider, err := catalog.Provider("stable")
	if err != nil {
		t.Fatalf("Original provider: %v", err)
	}
	if provider.Name != "Stable" {
		t.Fatalf("Caller mutation escaped observation: %q", provider.Name)
	}
}

func TestSourceReportsMalformedLocalSiblingAsDegradedObservation(t *testing.T) {
	root := t.TempDir()
	modelsDir := filepath.Join(root, "providers", "local-provider", "models")
	if err := os.MkdirAll(modelsDir, constants.DirPermissions); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	files := map[string]string{
		"providers.yaml": "- id: local-provider\n  name: Local Provider\n",
		filepath.Join("providers", "local-provider", "models", "valid.yaml"):   "id: valid\nname: Valid\nlimits:\n  context_window: 1\n",
		filepath.Join("providers", "local-provider", "models", "invalid.yaml"): "id: invalid\nname: [unterminated\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), constants.FilePermissions); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}

	observation, err := New(WithCatalogPath(root)).Observe(context.Background())
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if observation.Status != sources.ObservationStatusDegraded ||
		observation.Completeness != sources.ObservationCompletenessPartial ||
		observation.Records.Accepted != 1 ||
		observation.Records.Rejected != 1 {
		t.Fatalf("observation health = %#v", observation)
	}
	provider, err := observation.Catalog.Provider("local-provider")
	if err != nil {
		t.Fatalf("Provider: %v", err)
	}
	_, hasValid := provider.Models["valid"]
	_, hasInvalid := provider.Models["invalid"]
	if !hasValid || hasInvalid {
		t.Fatalf("models = %#v, want only valid sibling", provider.Models)
	}
}
