package permission

import (
	"context"
	"math"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// PublishCatalog prepares an ordinary permitted catalog and publishes its next durable authority sequence.
// Expected identifies the exact predecessor. An empty value requires an empty store.
// An exact retry may identify the prior predecessor after an ambiguous successful response.
//
// The caller authorizes this origin and applies its catalog policy before publication.
// A conflict requires a fresh operator or source decision. This method never retries a stale proposal automatically.
// The serving client must still activate the returned generation.
func (p *Publisher) PublishCatalog(ctx context.Context, input catalogs.Generation, expected string) (catalogs.Generation, error) {
	return p.publishCatalog(ctx, input, expected, false)
}

// BootstrapCatalog explicitly adopts an ordinary store and publishes its first authority generation.
// It requires the exact ordinary predecessor, or an empty expectation for an empty store.
// It cannot reset an established authority. Exact retries retain their original generation and sequence.
func (p *Publisher) BootstrapCatalog(ctx context.Context, input catalogs.Generation, expected string) (catalogs.Generation, error) {
	return p.publishCatalog(ctx, input, expected, true)
}

func (p *Publisher) publishCatalog(ctx context.Context, input catalogs.Generation, expected string, bootstrap bool) (catalogs.Generation, error) {
	candidate, err := p.PrepareCatalog(ctx, input, expected)
	if err != nil {
		return catalogs.Generation{}, err
	}
	if err := p.commit(ctx, candidate, expected, bootstrap); err != nil {
		return catalogs.Generation{}, err
	}
	return candidate, nil
}

// PrepareCatalog selects an authority sequence from the exact durable predecessor without writing it.
// The returned generation lets a caller bind its final identity to a recovery journal before Commit or Bootstrap.
// Preparation reserves no sequence. The final commit must use the same expectation and can still conflict.
// An ordinary predecessor requires Bootstrap even when preparation succeeds.
func (p *Publisher) PrepareCatalog(ctx context.Context, input catalogs.Generation, expected string) (catalogs.Generation, error) {
	if err := p.ready(ctx); err != nil {
		return catalogs.Generation{}, err
	}
	config := GenerationConfig{AuthorityID: p.config.AuthorityID, PolicyID: p.config.PolicyID, Sequence: 1}
	candidate, err := PrepareGeneration(input, config)
	if err != nil {
		return catalogs.Generation{}, err
	}
	current, err := p.store.Current(ctx)
	if err != nil && (!errors.IsNotFound(err) || expected != "") {
		return catalogs.Generation{}, err
	}
	if err == nil {
		candidate, err = p.prepareNextCatalog(input, candidate, current, expected)
		if err != nil {
			return catalogs.Generation{}, err
		}
	}
	return candidate, nil
}

func (p *Publisher) prepareNextCatalog(input, initial, current catalogs.Generation, expected string) (catalogs.Generation, error) {
	head := current.Manifest.AuthorityHead
	if head == (catalogs.CatalogAuthorityHead{}) {
		if current.Manifest.GenerationID != expected {
			return catalogs.Generation{}, publicationConflict(expected, current.Manifest.GenerationID)
		}
		return initial, nil
	}
	if err := p.validateHead(head); err != nil {
		return catalogs.Generation{}, err
	}
	if !head.SupportsPermissions() {
		return catalogs.Generation{}, publicationError("current authority requires unsupported permission semantics")
	}
	sequence := head.Sequence
	if current.Manifest.GenerationID == expected {
		if sequence == math.MaxUint64 {
			return catalogs.Generation{}, publicationError("authority publication sequence is exhausted")
		}
		sequence++
	}
	config := GenerationConfig{AuthorityID: p.config.AuthorityID, PolicyID: p.config.PolicyID, Sequence: sequence}
	candidate, err := prepareAuthorityManifest(input, config, initial.Manifest.AuthorityHead.RequiredPermissionRevision)
	if err != nil {
		return catalogs.Generation{}, err
	}
	if current.Manifest.GenerationID != expected && candidate.Manifest.GenerationID != current.Manifest.GenerationID {
		return catalogs.Generation{}, publicationConflict(expected, current.Manifest.GenerationID)
	}
	// Commit checks complete immutable equality for a retry, then fences every concurrent writer through the store.
	return candidate, nil
}

func publicationConflict(expected, actual string) error {
	return &errors.ConflictError{Resource: "catalog authority publication", Expected: expected, Actual: actual}
}
