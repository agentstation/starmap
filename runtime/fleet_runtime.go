package runtime

import (
	"context"
	"slices"

	"github.com/agentstation/starmap/internal/bootstrap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

// WithFleetStore selects shared catalog publication and refresh ownership together.
// The runtime stores acquisition inputs with each shared publication, outside its private directory.
// The fleet store takes precedence over stores passed through WithClientOptions.
func WithFleetStore(store FleetStore) Option {
	return func(config *options) error {
		if store == nil {
			return fleetConflict("a fleet store is required")
		}
		if config.fleetStore != nil || config.leaseStore != nil {
			return fleetConflict("select one fleet store without a separate lease store")
		}
		config.fleetStore = &fleetCommitStore{FleetStore: store}
		config.leaseStore = store
		return nil
	}
}

// WithFleetAuthorityOrigin selects one fleet store for origin publication and permission issuance.
// The adapter must support independent authority-head observations.
func WithFleetAuthorityOrigin(store FleetStore, origin OriginConfig) Option {
	return func(config *options) error {
		if _, ok := store.(storage.AuthorityHeadReader); !ok {
			return originError("the fleet store must support independent authority observations")
		}
		if err := WithFleetStore(store)(config); err != nil {
			return err
		}
		return WithAuthorityOrigin(config.fleetStore, origin)(config)
	}
}

// FleetStatus describes the accepted publication and this replica's ability to replay its inputs.
// Replay readiness does not establish current lease ownership or inference permission.
type FleetStatus struct {
	Head        FleetHead
	ReplayReady bool
}

// FleetStatus returns local fleet state without a storage operation.
func (r *Runtime) FleetStatus() (FleetStatus, bool) {
	if r == nil || r.config.fleetStore == nil {
		return FleetStatus{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return FleetStatus{Head: r.fleetHead, ReplayReady: r.fleetReplayError == nil}, true
}

func (config options) readFleetBootstrap(ctx context.Context) (context.Context, *FleetSnapshot, error) {
	if config.fleetStore == nil {
		return ctx, nil, nil
	}
	snapshot, err := config.fleetStore.CurrentPublication(ctx)
	if errors.IsNotFound(err) {
		return config.fleetStore.readContext(ctx, nil), nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if err := snapshot.Validate(); err != nil {
		return nil, nil, err
	}
	return config.fleetStore.readContext(ctx, &snapshot), &snapshot, nil
}

func (r *Runtime) initializeFleetLayers(ctx context.Context, snapshot *FleetSnapshot) error {
	publisher, err := r.instanceIdentity()
	if err != nil {
		return err
	}
	manifest, err := bootstrap.GenerationManifest()
	if err != nil {
		return err
	}
	r.layers = layerSet{publisherID: publisher, publisherAliases: slices.Clone(r.config.source.Aliases),
		embedded: r.client.EmbeddedCatalogState(), embeddedManifest: &manifest, requireAuthority: r.requiresAuthority(),
		providerBindings: r.config.providerBindings, acquisitionSources: r.config.acquisitionSources}
	r.layers.sourceConfiguration = slices.Clone(r.config.sourceConfiguration)
	if snapshot == nil {
		return ctx.Err()
	}
	layers, pin, replayErr := r.replayFleetSnapshot(ctx, *snapshot, r.layers)
	if err := ctx.Err(); err != nil {
		return err
	}
	r.fleetHead, r.fleetReplayError = snapshot.Head, replayErr
	if replayErr == nil {
		r.layers = layers
		r.pinRecord = pin
	}
	return nil
}

func (r *Runtime) replayFleetSnapshot(ctx context.Context, snapshot FleetSnapshot, local layerSet) (layerSet, *generationPinRecord, error) {
	layers, pin, err := recoverFleetState(ctx, snapshot, local)
	if err != nil {
		return layerSet{}, nil, err
	}
	if layers.source != nil && r.source != nil && layers.source.Identity != r.source.Identity() {
		return layerSet{}, nil, fleetConflict("retained inputs belong to a different configured source")
	}
	if pin != nil && pin.Binding != r.pinBinding() {
		return layerSet{}, nil, pinRecordConflict("the shared pin belongs to a different source or authority")
	}
	return layers, pin, nil
}

type fleetGrantContextKey struct{}

func (r *Runtime) captureFleetGrant(ctx context.Context, epoch uint64) (context.Context, error) {
	if r.config.fleetStore == nil {
		return ctx, nil
	}
	grant, err := r.lease.grant(epoch)
	if err != nil {
		return nil, err
	}
	if err := grant.validateFleet(); err != nil {
		return nil, err
	}
	return context.WithValue(ctx, fleetGrantContextKey{}, grant), nil
}

func (r *Runtime) prepareFleetCommit(ctx context.Context, epoch uint64, layers layerSet) (context.Context, *fleetCommit, error) {
	return r.prepareFleetCommitWithPin(ctx, epoch, layers, nil)
}

func (r *Runtime) prepareFleetCommitWithPin(ctx context.Context, epoch uint64, layers layerSet, pin *generationPinRecord) (context.Context, *fleetCommit, error) {
	if r.config.fleetStore == nil {
		return ctx, nil, nil
	}
	original, ok := ctx.Value(fleetGrantContextKey{}).(Lease)
	if !ok {
		return nil, nil, fleetConflict("publication requires the grant captured before acquisition")
	}
	current, err := r.lease.grant(epoch)
	if err != nil {
		return nil, nil, err
	}
	if !sameLeaseGrant(original, current) {
		return nil, nil, fleetConflict("the acquisition grant no longer matches local ownership")
	}
	r.mu.RLock()
	head, replayErr := r.fleetHead, r.fleetReplayError
	r.mu.RUnlock()
	if replayErr != nil {
		return nil, nil, replayErr
	}
	if pin != nil {
		accepted := *pin
		if accepted.Phase == pinPrepared {
			accepted.Phase, accepted.Receipt.AcceptedAt = pinAccepted, r.config.now().UTC()
		}
		pin = &accepted
	}
	return r.config.fleetStore.prepareWithPin(ctx, original, head, layers, pin)
}

func (r *Runtime) finishFleetCommit(ctx context.Context, attempt *fleetCommit) error {
	if attempt == nil {
		return nil
	}
	head := attempt.result()
	if head == (FleetHead{}) {
		current := r.client.CurrentCatalogState()
		if attempt.expected.GenerationID == current.GenerationID && attempt.expected.RecoveryChecksum == attempt.checksum {
			return nil
		}
		generation, err := r.client.CurrentGeneration(ctx)
		if err != nil {
			return err
		}
		if _, err := r.client.Activate(r.authorityPublicationContext(ctx), generation); err != nil {
			return err
		}
		head = attempt.result()
	}
	if head == (FleetHead{}) {
		return fleetConflict("publication returned without an accepted fleet head")
	}
	r.mu.Lock()
	r.fleetHead = head
	r.mu.Unlock()
	return nil
}

// refreshFleetInputs selects one coherent shared snapshot before acquisition or follower activation.
func (r *Runtime) refreshFleetInputs(ctx context.Context, requireReplay bool) error {
	if r.config.fleetStore == nil {
		return nil
	}
	r.publicationMu.Lock()
	defer r.publicationMu.Unlock()
	head, err := r.config.fleetStore.CurrentHead(ctx)
	if errors.IsNotFound(err) {
		r.mu.RLock()
		head := r.fleetHead
		r.mu.RUnlock()
		if head == (FleetHead{}) {
			return nil
		}
		return fleetConflict("the previously accepted fleet head is missing")
	}
	if err != nil {
		return err
	}
	if err := head.Validate(); err != nil {
		return err
	}
	r.mu.RLock()
	local, prior, previousError := r.layers, r.fleetHead, r.fleetReplayError
	r.mu.RUnlock()
	if prior != (FleetHead{}) && (head.Identity != prior.Identity || head.Revision < prior.Revision || (head.Revision == prior.Revision && head != prior)) {
		return fleetConflict("the shared publication regressed or changed recovery identity")
	}
	if head == prior {
		if requireReplay {
			return previousError
		}
		return nil
	}
	snapshot, err := r.config.fleetStore.Publication(ctx, head)
	if err != nil {
		return err
	}
	if snapshot.Head != head {
		return fleetConflict("the snapshot differs from the selected publication head")
	}
	if err := snapshot.Validate(); err != nil {
		return err
	}
	layers, pin, replayErr := r.replayFleetSnapshot(ctx, snapshot, local)
	readCtx := r.config.fleetStore.readContext(r.authorityPublicationContext(ctx), &snapshot)
	if r.config.generationPin != "" {
		if pin == nil || pin.Receipt.SelectedGenerationID != r.config.generationPin {
			return pinRecordConflict("the fleet publication differs from the configured pin")
		}
		readCtx = context.WithValue(readCtx, generationPinContextKey{}, r.config.pinCapability)
	}
	_, err = r.client.Reload(readCtx, func(generation catalogs.Generation) error {
		return r.config.validateStoredAuthoritySelection(generation.Manifest.AuthorityHead)
	})
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.fleetHead, r.fleetReplayError = snapshot.Head, replayErr
	if replayErr == nil {
		r.layers = layers
		r.pinRecord = pin
	}
	if pin != nil && r.config.generationPin == pin.Receipt.SelectedGenerationID {
		generation := snapshot.Publication.Generation
		r.pinnedSource = &sourceLayer{GenerationID: generation.Manifest.GenerationID, Payload: generation.Payload,
			Manifest: &generation.Manifest, PublishedAt: generation.Manifest.GeneratedAt}
	}
	r.effective = r.client.CurrentCatalogState()
	r.activateAuthorityLocked(r.selectedAuthoritySource())
	state := r.effective
	r.mu.Unlock()
	if snapshot.Head != prior {
		r.broadcast(state)
	}
	if requireReplay {
		return replayErr
	}
	return nil
}

// RefreshFleet activates the durable shared head after a missed notification or reconnect.
// It queries no source and requires no refresh lease.
func (r *Runtime) RefreshFleet(ctx context.Context) error {
	if r == nil || r.config.fleetStore == nil {
		return fleetConflict("the runtime has no fleet store")
	}
	_, err := r.execute(ctx, runKindAccepted, func(ctx context.Context, _ *RefreshReport, _ uint64) error {
		return r.refreshFleetInputs(ctx, false)
	})
	return err
}
