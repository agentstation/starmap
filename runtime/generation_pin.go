package runtime

import (
	"context"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// WithGenerationPin selects one retained generation for this runtime's lifetime.
// The host persists this setting in its configuration authority. An empty value clears it.
// Replacing the runtime applies a changed pin. Permission observation remains active.
func WithGenerationPin(id string) Option {
	return func(config *options) error {
		if id != strings.TrimSpace(id) {
			return &errors.ValidationError{Field: "catalog_generation_pin", Message: "must not contain surrounding whitespace"}
		}
		config.generationPin = id
		return nil
	}
}

type generationPinContextKey struct{}
type generationPinCapability struct{ owned bool }

func (config options) generationPinGuard(ctx context.Context) error {
	supplied, _ := ctx.Value(generationPinContextKey{}).(*generationPinCapability)
	if supplied == nil || supplied != config.pinCapability || !supplied.owned {
		return pinnedGenerationError()
	}
	return nil
}

func pinnedGenerationError() error {
	return &errors.ConflictError{Resource: "catalog generation pin", Message: "replace or clear the configured pin before changing the catalog"}
}

func (r *Runtime) validateGenerationMutation() error {
	if r.config.generationPin != "" {
		return pinnedGenerationError()
	}
	return nil
}

// initializeGenerationPin reads only the selected store and validates the exact artifact.
// Retained inputs remain available for an explicit unpin and cannot rebuild this selection.
func (r *Runtime) initializeGenerationPin(ctx context.Context) error {
	generation, err := r.client.Generation(ctx, r.config.generationPin)
	if err != nil {
		return err
	}
	if generation.Manifest.GenerationID != r.config.generationPin {
		return &errors.ValidationError{Field: "catalog_generation_pin", Message: "retained generation identity differs from the requested pin"}
	}
	catalog, err := catalogs.DecodeCatalogGeneration(generation)
	if err != nil {
		return err
	}
	head := generation.Manifest.AuthorityHead
	if err := r.config.validateStoredAuthoritySelection(head); err != nil {
		return err
	}
	if r.requiresAuthority() && (head.AuthorityID != r.config.source.AuthorityID || head.PolicyID != r.config.source.PolicyID || !head.SupportsPermissions()) {
		return &errors.ValidationError{Field: "catalog_generation_pin", Message: "requires a generation from the configured authority and policy"}
	}
	if r.config.origin != nil {
		if err := r.config.origin.validateCurrent(generation); err != nil {
			return err
		}
		if generation.Manifest.GenerationID != r.client.CurrentGenerationID() {
			return &errors.ConflictError{Resource: "origin generation pin", Message: "publish a new authority revision before pinning a prior catalog"}
		}
	}
	r.pinnedSource = &sourceLayer{GenerationID: generation.Manifest.GenerationID, Payload: generation.Payload,
		Manifest: &generation.Manifest, PublishedAt: generation.Manifest.GeneratedAt}
	current := r.client.CurrentCatalogState()
	r.effective = starmap.CatalogState{Catalog: catalog, GenerationID: generation.Manifest.GenerationID,
		PayloadChecksum: generation.Manifest.Payload.Checksum, GeneratedAt: generation.Manifest.GeneratedAt,
		AuthorityHead: head, Sequence: current.Sequence}
	r.report.startedAt = r.config.now()
	return nil
}

// publishGenerationPin aligns the serving client before Open exposes the runtime.
// The private capability does not reach acquisition callbacks or callers of Client.
func (r *Runtime) publishGenerationPin(ctx context.Context) error {
	if r.client.CurrentGenerationID() != r.config.generationPin {
		if err := r.lease.fence(r.lease.epoch()); err != nil {
			return err
		}
		pinContext := context.WithValue(r.authorityPublicationContext(ctx), generationPinContextKey{}, r.config.pinCapability)
		if _, err := r.client.Rollback(pinContext, r.config.generationPin); err != nil {
			return err
		}
	}
	r.mu.Lock()
	r.effective = r.client.CurrentCatalogState()
	r.activateAuthorityLocked(r.pinnedSource)
	r.mu.Unlock()
	return nil
}

func (r *Runtime) selectedAuthoritySource() *sourceLayer {
	if r.pinnedSource != nil {
		return r.pinnedSource
	}
	return r.layers.source
}
