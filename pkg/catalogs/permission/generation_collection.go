package permission

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

// GenerationCollector forwards collection while checking the current authority identity.
// The underlying collector still protects current, required, and leased generations atomically.
func (p *Publisher) GenerationCollector() (storage.GenerationCollector, bool) {
	if p == nil || nilPublicationStore(p.store) {
		return nil, false
	}
	collector, ok := storage.GenerationCollectorFor(p.store)
	if !ok {
		return nil, false
	}
	return publisherGenerationCollector{publisher: p, collector: collector}, true
}

type publisherGenerationCollector struct {
	publisher *Publisher
	collector storage.GenerationCollector
}

func (c publisherGenerationCollector) Collect(ctx context.Context, request storage.RetentionRequest) (storage.RetentionReport, error) {
	if err := c.publisher.ready(ctx); err != nil {
		return storage.RetentionReport{}, err
	}
	current, err := c.publisher.Current(ctx)
	if err != nil {
		if !errors.IsNotFound(err) || request.ExpectedGenerationID != "" {
			return storage.RetentionReport{}, err
		}
	} else {
		if err := current.Validate(); err != nil {
			return storage.RetentionReport{}, err
		}
		if err := c.publisher.validateHead(current.Manifest.AuthorityHead); err != nil {
			return storage.RetentionReport{}, err
		}
		if !current.Manifest.AuthorityHead.SupportsPermissions() {
			return storage.RetentionReport{}, publicationError("current authority requires unsupported permission semantics")
		}
	}
	return c.collector.Collect(ctx, request)
}

var _ storage.GenerationCollectionProvider = (*Publisher)(nil)
