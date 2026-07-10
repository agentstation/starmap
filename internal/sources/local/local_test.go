package local

import (
	"context"
	"sync"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

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
