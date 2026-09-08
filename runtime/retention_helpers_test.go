package runtime

import (
	"context"
	"github.com/agentstation/starmap"
)

// rebuild exercises production reconstruction without new input observations.
func (r *Runtime) rebuild(ctx context.Context, epoch uint64) (starmap.CatalogState, error) {
	return r.publishInputChanges(ctx, nil, nil, epoch)
}

// retainProviders seeds durable test fixtures before catalog publication.
// Production retention uses the input publication transaction.
func (r *Runtime) retainProviders(ctx context.Context, layers []ProviderLayer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	prepared, err := prepareProviderEvidence(layers)
	if err != nil {
		return err
	}
	r.providerRetentionMu.Lock()
	defer r.providerRetentionMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.RLock()
	selected, err := selectProviderEvidence(prepared, r.layers.providers)
	r.mu.RUnlock()
	if err != nil {
		return err
	}
	for _, layer := range selected {
		if err := r.store.saveProvider(ctx, layer); err != nil {
			return err
		}
		r.mu.Lock()
		r.layers.setProvider(layer)
		r.mu.Unlock()
	}
	return nil
}
