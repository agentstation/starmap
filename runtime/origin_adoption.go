package runtime

import (
	"context"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// Accepted-store polling remains active when an operator disables public source polling.
const originAcceptedPollInterval = 30 * time.Second

func (r *Runtime) refreshOriginAccepted(ctx context.Context) error {
	if r.config.origin == nil {
		return originError("accepted origin refresh requires an origin")
	}
	_, err := r.execute(ctx, runKindAccepted, func(ctx context.Context, report *RefreshReport, _ uint64) error {
		r.publicationMu.Lock()
		defer r.publicationMu.Unlock()
		observed, err := r.config.origin.publisher.CurrentAuthorityHead(ctx)
		if err != nil {
			return err
		}
		if observed == r.State().AuthorityHead && r.authorityClientMatches(r.State()) {
			return nil
		}
		publication, err := r.client.Reload(r.authorityPublicationContext(ctx), func(generation catalogs.Generation) error {
			if err := r.config.origin.validateHead(generation.Manifest.AuthorityHead); err != nil {
				return err
			}
			return observed.ValidateSuccessor(generation.Manifest.AuthorityHead)
		})
		if err != nil {
			return err
		}
		state := r.client.CurrentCatalogState()
		r.mu.Lock()
		r.effective = state
		r.originFollowed = true
		r.mu.Unlock()
		report.Source.Published = publication.Published
		report.Source.GenerationID = publication.GenerationID
		if publication.Published {
			r.broadcast(state)
		}
		return nil
	})
	return err
}

// A follower must reconstruct the accepted inputs before it can take publication ownership.
// Shared input recovery belongs to the fleet storage contract. An effective catalog alone is insufficient.
func (r *Runtime) validateOriginTakeover(ctx context.Context) error {
	r.mu.RLock()
	followed := r.originFollowed
	inputs := r.layers
	r.mu.RUnlock()
	if r.config.origin == nil || !followed {
		return nil
	}
	current, err := r.config.origin.publisher.Current(ctx)
	if err != nil {
		return err
	}
	if err := r.config.origin.validateCurrent(current); err != nil {
		return err
	}
	local, err := inputs.build(ctx, inputs.embedded)
	if err != nil {
		return err
	}
	matches, err := r.config.origin.matchesSource(current, local)
	if err != nil {
		return err
	}
	if !matches {
		return &errors.ConflictError{Resource: "origin acquisition inputs", Message: "accepted generation requires retained input recovery before this replica can acquire publication ownership"}
	}
	return nil
}
