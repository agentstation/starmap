package runtime

import (
	"context"

	"github.com/agentstation/starmap"
)

func validateMigrationCatalog(ctx context.Context, directory string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	store, err := existingLayerStore(directory)
	if err != nil {
		return err
	}
	source, err := store.loadSource()
	if err != nil {
		return err
	}
	providers, err := store.loadProviders()
	if err != nil {
		return err
	}
	client, err := starmap.NewContext(ctx)
	if err != nil {
		return err
	}
	layers := layerSet{source: source, providers: providers}
	if _, err := layers.build(ctx, client.EmbeddedCatalogState()); err != nil {
		return err
	}
	return ctx.Err()
}
