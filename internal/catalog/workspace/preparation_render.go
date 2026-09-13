package workspace

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func renderPreparation(ctx context.Context, tree *preparationTree, catalog *catalogs.Catalog, identity Identity) error {
	builder, err := catalogs.NewBuilderFrom(catalog)
	if err != nil {
		return err
	}
	if err := builder.WriteYAML(func(name string, data []byte) error { return tree.writeFile(ctx, name, data) }); err != nil {
		return err
	}
	data, err := EncodeEndpointProjection(catalog, identity)
	if err != nil {
		return err
	}
	return tree.writeFile(ctx, endpointProjectionFilename, data)
}
