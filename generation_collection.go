package starmap

import (
	"context"
	"slices"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

// CanCollectGenerations reports whether the selected store supports coordinated collection.
// It reads no storage and starts no work.
func (c *Client) CanCollectGenerations() bool {
	_, ok := c.generationCollector()
	return ok
}

func (c *Client) generationCollector() (storage.GenerationCollector, bool) {
	if c == nil || c.options == nil || isNilCatalogStore(c.options.catalogStore) {
		return nil, false
	}
	return storage.GenerationCollectorFor(c.options.catalogStore)
}

// CollectGenerations applies explicit retention without changing the served catalog.
// It serializes with client updates and protects served and stored embedded generations.
// The caller supplies other required IDs and the expected stored generation.
// Collection remains available when a publication guard prevents catalog changes.
func (c *Client) CollectGenerations(ctx context.Context, request storage.RetentionRequest) (storage.RetentionReport, error) {
	if ctx == nil {
		return storage.RetentionReport{}, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	if err := ctx.Err(); err != nil {
		return storage.RetentionReport{}, err
	}
	collector, ok := c.generationCollector()
	if !ok {
		return storage.RetentionReport{}, &errors.ConfigError{Component: "catalog generation retention", Message: "selected store does not support coordinated collection"}
	}
	release, err := c.updates.acquire(ctx)
	if err != nil {
		return storage.RetentionReport{}, err
	}
	defer release()
	c.mu.RLock()
	active, embedded := c.generationID, c.embeddedBootstrap.GenerationID
	c.mu.RUnlock()
	request.RequiredGenerationIDs = slices.Clone(request.RequiredGenerationIDs)
	if active != "" {
		request.RequiredGenerationIDs = append(request.RequiredGenerationIDs, active)
	}
	if embedded != "" {
		_, err := c.options.catalogStore.Get(ctx, embedded)
		if err == nil {
			request.RequiredGenerationIDs = append(request.RequiredGenerationIDs, embedded)
		} else if !errors.IsNotFound(err) {
			return storage.RetentionReport{}, err
		} else if active == "" && request.ExpectedGenerationID == embedded {
			request.ExpectedGenerationID = ""
		}
	}
	slices.Sort(request.RequiredGenerationIDs)
	request.RequiredGenerationIDs = slices.Compact(request.RequiredGenerationIDs)
	return collector.Collect(ctx, request)
}
