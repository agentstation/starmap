package runtime

import (
	"context"

	"github.com/agentstation/starmap"
)

// retainEmbeddedStartup stores an unchanged compiled baseline during ordinary
// runtime startup. Root client construction remains read-only. Fleet, authority,
// origin, and pin publication retain their separate authorization contracts.
func (r *Runtime) retainEmbeddedStartup(ctx context.Context, state starmap.CatalogState) error {
	if r.config.fleetStore != nil || r.lease.status() == leaseLost || !r.client.PublishesDurably() || !r.client.Readiness().Embedded.Active {
		return nil
	}
	baseline := r.client.EmbeddedCatalogState()
	if state.GenerationID != baseline.GenerationID || state.PayloadChecksum != baseline.PayloadChecksum ||
		!state.GeneratedAt.Equal(baseline.GeneratedAt) || state.AuthorityHead != baseline.AuthorityHead {
		return nil
	}
	generation, err := r.client.CurrentGeneration(ctx)
	if err != nil {
		return err
	}
	if err := r.lease.fence(r.lease.epoch()); err != nil {
		return err
	}
	// Activate preserves the immutable identity and bytes. The store's CAS is
	// idempotent on restart and refuses a conflicting retained head.
	if _, err := r.client.Activate(ctx, generation); err != nil {
		return err
	}
	r.mu.Lock()
	r.effective = r.client.CurrentCatalogState()
	r.mu.Unlock()
	return nil
}
