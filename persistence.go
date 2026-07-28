package starmap

import (
	"context"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/save"
)

// Save atomically materializes the current committed generation into a YAML
// workspace. It never publishes a new generation.
func (c *Client) Save(opts ...save.Option) error {
	options := save.Defaults().Apply(opts...)
	writePath := options.Path()
	if writePath == "" && c.options != nil {
		writePath = c.options.catalogPath
	}
	if c.options != nil {
		if err := validateCatalogLayout(c.options.catalogStore, writePath); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultCatalogProjectionTimeout)
	defer cancel()
	release, err := c.updates.acquire(ctx)
	if err != nil {
		return err
	}
	defer release()

	state := c.CurrentCatalogState()
	generation, err := c.CurrentGeneration(ctx)
	if err != nil {
		return errors.WrapResource("get", "current catalog generation", "", err)
	}

	if _, err := workspace.Project(
		ctx,
		writePath,
		state.Catalog,
		workspace.Identity{
			GenerationID:    generation.Manifest.GenerationID,
			PayloadChecksum: generation.Manifest.Payload.Checksum,
		},
	); err != nil {
		return errors.WrapIO("write", "catalog", err)
	}

	return nil
}
