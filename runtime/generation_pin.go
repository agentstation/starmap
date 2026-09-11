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
	target := catalogs.Generation{Manifest: r.pinnedSource.Manifest.Copy(), Payload: r.pinnedSource.Payload}
	record := r.pinRecord
	if record != nil && (record.Phase == pinReleased || record.Receipt.SelectedGenerationID != r.config.generationPin) {
		record = nil
	}
	current := r.client.CurrentCatalogState()
	if record != nil && record.Receipt.PayloadChecksum != target.Manifest.Payload.Checksum {
		return pinRecordConflict("the selected payload differs from its acceptance record")
	}
	if record != nil && record.Phase == pinAccepted && current.GenerationID != record.Receipt.AcceptedGenerationID {
		return pinRecordConflict("the catalog changed after pin acceptance")
	}
	if record != nil && current.GenerationID != record.Receipt.PreviousGenerationID && current.GenerationID != record.Receipt.AcceptedGenerationID {
		return pinRecordConflict("the catalog differs from both recorded publication states")
	}
	generation, err := r.preparePinGeneration(ctx, target, record)
	if err != nil {
		return err
	}
	if record != nil && !pinRecordMatches(*record, generation) {
		return pinRecordConflict("the prepared artifact differs from the recorded selection")
	}
	if record == nil {
		operationID, err := r.client.NextID()
		if err != nil {
			return err
		}
		record = &generationPinRecord{Version: generationPinRecordVersion, Binding: r.pinBinding(), Phase: pinPrepared,
			Receipt: GenerationPinAcceptance{OperationID: operationID, SelectedGenerationID: r.config.generationPin,
				AcceptedGenerationID: generation.Manifest.GenerationID, PreviousGenerationID: current.GenerationID,
				PayloadChecksum: generation.Manifest.Payload.Checksum, AuthorityHead: generation.Manifest.AuthorityHead, RequestedAt: r.config.now().UTC()}}
		if err := r.store.savePinRecord(ctx, *record); err != nil {
			return err
		}
	}
	pendingPublication := record.Phase == pinPrepared && record.Receipt.PreviousGenerationID != record.Receipt.AcceptedGenerationID
	if current.GenerationID != generation.Manifest.GenerationID || pendingPublication {
		if err := r.lease.fence(r.lease.epoch()); err != nil {
			return err
		}
		pinContext := context.WithValue(r.authorityPublicationContext(ctx), generationPinContextKey{}, r.config.pinCapability)
		if r.config.origin != nil {
			if _, err := r.client.Activate(pinContext, generation); err != nil {
				return err
			}
		} else if _, err := r.client.Rollback(pinContext, generation.Manifest.GenerationID); err != nil {
			return err
		}
	}
	active := r.client.CurrentCatalogState()
	if active.GenerationID != record.Receipt.AcceptedGenerationID || active.PayloadChecksum != record.Receipt.PayloadChecksum || active.AuthorityHead != record.Receipt.AuthorityHead {
		return pinRecordConflict("activation differs from the recorded selection")
	}
	accepted := *record
	if record.Phase != pinAccepted {
		accepted.Phase, accepted.Receipt.AcceptedAt = pinAccepted, r.config.now().UTC()
	}
	// Reassert the same receipt to confirm durability after a possible directory flush failure.
	if err := r.store.savePinRecord(ctx, accepted); err != nil {
		return err
	}
	record = &accepted
	r.mu.Lock()
	r.pinRecord = record
	r.pinnedSource = &sourceLayer{GenerationID: generation.Manifest.GenerationID, Manifest: &generation.Manifest,
		Payload: generation.Payload, PublishedAt: generation.Manifest.GeneratedAt}
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
