package starmap

import (
	"context"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/pkg/errors"
)

// Save atomically materializes the current committed generation into a YAML
// workspace configured at construction. It never publishes a new generation.
func (c *Client) Save() error {
	return c.saveTo("")
}

// SaveTo atomically materializes the current committed generation into path.
// It never publishes a new generation.
func (c *Client) SaveTo(path string) error {
	return c.saveTo(path)
}

func (c *Client) saveTo(writePath string) error {
	if writePath == "" && c.options != nil {
		writePath = c.options.catalogPath
	}
	if c.options != nil {
		if err := validateCatalogLayout(c.options.catalogStore, writePath); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), catalogProjectionTimeout)
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
