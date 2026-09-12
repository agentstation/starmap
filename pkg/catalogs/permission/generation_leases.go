package permission

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

// GenerationLeaser forwards optional read leases without exposing an unguarded writer.
func (p *Publisher) GenerationLeaser() (storage.GenerationLeaser, bool) {
	if p == nil || nilPublicationStore(p.store) {
		return nil, false
	}
	leaser, ok := storage.GenerationLeaserFor(p.store)
	if !ok {
		return nil, false
	}
	return publisherGenerationLeaser{publisher: p, leaser: leaser}, true
}

type publisherGenerationLeaser struct {
	publisher *Publisher
	leaser    storage.GenerationLeaser
}

func (l publisherGenerationLeaser) AcquireGeneration(ctx context.Context, id string) (catalogs.Generation, func() error, error) {
	if err := l.publisher.ready(ctx); err != nil {
		return catalogs.Generation{}, nil, err
	}
	return l.leaser.AcquireGeneration(ctx, id)
}

var _ storage.GenerationLeaseProvider = (*Publisher)(nil)
