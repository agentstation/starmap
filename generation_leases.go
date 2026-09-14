package starmap

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"

	bootstraploader "github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

// CanLeaseGenerations reports whether the selected store supports generation read leases.
// It reads no storage and starts no background work.
func (c *Client) CanLeaseGenerations() bool {
	_, ok := c.generationLeaser()
	return ok
}

func (c *Client) generationLeaser() (storage.GenerationLeaser, bool) {
	if c == nil || c.options == nil || isNilCatalogStore(c.options.catalogStore) {
		return nil, false
	}
	return storage.GenerationLeaserFor(c.options.catalogStore)
}

// AcquireGeneration reads and protects a stored generation until release.
// The selected store must support read leases. A missing embedded generation
// returns the verified compiled artifact. Compiled bytes need no storage lease.
// The caller must release every successful acquisition, even after cancellation.
func (c *Client) AcquireGeneration(ctx context.Context, id string) (catalogs.Generation, func() error, error) {
	if c == nil || c.options == nil {
		return catalogs.Generation{}, nil, &errors.ValidationError{Field: "starmap.client", Message: "is required"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return catalogs.Generation{}, nil, err
	}
	leaser, ok := c.generationLeaser()
	if !ok {
		return catalogs.Generation{}, nil, &errors.ConfigError{Component: "catalog generation lease", Message: "selected store does not support generation read leases"}
	}
	leased, release, err := leaser.AcquireGeneration(ctx, id)
	if err != nil {
		if release != nil {
			return catalogs.Generation{}, nil, stderrors.Join(err, release())
		}
		if errors.IsNotFound(err) {
			return c.acquireEmbeddedGeneration(ctx, id)
		}
		return catalogs.Generation{}, nil, err
	}
	if release == nil {
		return catalogs.Generation{}, nil, &errors.ValidationError{Field: "catalog generation lease", Message: "release function is required"}
	}
	// Read through the configured store so forwarded capabilities preserve its policy.
	generation, err := c.options.catalogStore.Get(ctx, id)
	if err == nil {
		err = validateGenerationLease(id, leased, generation)
	}
	if err != nil {
		return catalogs.Generation{}, nil, stderrors.Join(err, release())
	}
	return generation, release, nil
}

func (c *Client) acquireEmbeddedGeneration(ctx context.Context, id string) (catalogs.Generation, func() error, error) {
	c.mu.RLock()
	embeddedID := c.embeddedBootstrap.GenerationID
	c.mu.RUnlock()
	if id != embeddedID {
		return catalogs.Generation{}, nil, &errors.NotFoundError{Resource: "catalog generation", ID: id}
	}
	if err := ctx.Err(); err != nil {
		return catalogs.Generation{}, nil, err
	}
	compiled, err := bootstraploader.Generation()
	if err != nil {
		return catalogs.Generation{}, nil, err
	}
	generation, err := c.Generation(ctx, id)
	if err == nil {
		err = validateGenerationLease(id, compiled, generation)
	}
	if err != nil {
		return catalogs.Generation{}, nil, err
	}
	return generation, func() error { return nil }, nil
}

func validateGenerationLease(id string, leased, selected catalogs.Generation) error {
	for _, generation := range []catalogs.Generation{leased, selected} {
		if generation.Manifest.GenerationID != id {
			return &errors.ValidationError{Field: "catalog generation lease", Message: "generation identity does not match the requested ID"}
		}
		if err := generation.Validate(); err != nil {
			return err
		}
	}
	leasedManifest, err := json.Marshal(leased.Manifest)
	if err != nil {
		return errors.WrapResource("encode", "leased generation manifest", id, err)
	}
	selectedManifest, err := json.Marshal(selected.Manifest)
	if err != nil {
		return errors.WrapResource("encode", "selected generation manifest", id, err)
	}
	if !bytes.Equal(leased.Payload, selected.Payload) || !bytes.Equal(leasedManifest, selectedManifest) {
		return &errors.ValidationError{Field: "catalog generation lease", Message: "configured read differs from the protected generation"}
	}
	return nil
}
