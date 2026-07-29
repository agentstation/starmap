// Package consumer is a real external module exercising explicit,
// provider-independent catalog publication through a caller-owned store.
package consumer

import (
	"context"
	"fmt"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
)

// Publish proves that a Go consumer can compose durable catalog mutation
// without importing source acquisition, provider clients, or server code.
func Publish(ctx context.Context) error {
	store := catalogstore.NewMemory()
	sm, err := starmap.NewContext(ctx, starmap.WithCatalogStore(store))
	if err != nil {
		return err
	}

	publication, err := sm.Update(ctx, func(
		_ context.Context,
		current *catalogs.Catalog,
	) (*starmap.Candidate, error) {
		builder, err := catalogs.NewBuilderFrom(current)
		if err != nil {
			return nil, err
		}
		if err := builder.SetProvider(catalogs.Provider{
			ID:   "external-store",
			Name: "External Store",
		}); err != nil {
			return nil, err
		}
		catalog, err := builder.Build()
		if err != nil {
			return nil, err
		}
		return starmap.NewCandidate(catalog)
	})
	if err != nil {
		return err
	}
	if !publication.Published {
		return fmt.Errorf("expected publication")
	}

	generation, err := store.Current(ctx)
	if err != nil {
		return err
	}
	if generation.Manifest.GenerationID != publication.GenerationID {
		return fmt.Errorf(
			"store generation %q does not match publication %q",
			generation.Manifest.GenerationID,
			publication.GenerationID,
		)
	}
	if _, err := sm.Catalog().Provider("external-store"); err != nil {
		return err
	}
	return nil
}
