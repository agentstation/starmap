// Package runtime holds the connected catalog runtime. It opens a Starmap
// client, reads verified upstream generations from one source, publishes each
// accepted generation, and reports the operator-facing status of that work.
//
// The default source is the attested public GitHub channel. A caller that
// opens the runtime accepts the signature verification and the network reads
// that this channel needs. The root starmap package stays the offline library.
package runtime

import (
	"context"
	stderrors "errors"
	"sync"
	"time"

	"github.com/gofrs/flock"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

const (
	// closeJoinTimeout bounds how long Close waits for runtime-owned work.
	closeJoinTimeout = 5 * time.Second

	// updatesBuffer bounds the retained catalog-state notifications. A slow
	// reader loses intermediate states, never the newest one.
	updatesBuffer = 16
)

// ProviderLayer is one retained per-provider observation. The runtime keeps
// the last-known-good layer of every provider, so one failing provider never
// removes its records from the effective catalog.
type ProviderLayer struct {
	// ProviderID names the observed provider.
	ProviderID catalogs.ProviderID

	// Payload is the canonical encoding of the provider observation.
	Payload []byte

	// Digest is the content digest of Payload.
	Digest string

	// ObservedAt is when acquisition accepted the observation.
	ObservedAt time.Time

	// Receipt binds source identity, completeness, and classified issues to Payload.
	Receipt sources.ObservationReceipt
}

// AcquisitionRequest describes one provider acquisition run.
type AcquisitionRequest struct {
	// RunID is the run identity that logs and status report.
	RunID string

	// Current is the immutable catalog the run starts from.
	Current *catalogs.Catalog

	// Providers restricts the run. An empty list observes every eligible
	// provider.
	Providers []catalogs.ProviderID

	// CoalesceWindow bounds how long completed observations wait for a slower
	// sibling before they publish.
	CoalesceWindow time.Duration

	// Publish emits the layers that completed inside one coalescing window.
	// The runtime retains them, rebuilds the effective catalog, and publishes
	// one generation under the lease epoch of the run. An acquirer that emits
	// nothing early leaves the field unused, and the runtime then publishes
	// every layer once the run returns.
	Publish func(context.Context, []ProviderLayer) error
}

// AcquisitionResult is what one acquisition run observed.
type AcquisitionResult struct {
	// Eligible counts provider targets, or bindings when the active set is explicit.
	Eligible int

	// Attempts holds one terminal attempt per eligible provider or binding.
	Attempts []sources.ProviderAttempt

	// Layers holds one successful observation per provider or binding.
	Layers []ProviderLayer
}

// Acquirer collects provider observations for the runtime. The root package
// selects no concrete provider client, so the deployment injects this role.
// Package acquisition supplies the built-in composition.
type Acquirer interface {
	AcquireProviders(ctx context.Context, request AcquisitionRequest) (AcquisitionResult, error)
}

// Runtime is a connected Starmap. It serves the embedded catalog immediately,
// refreshes from one selected upstream source, retains per-provider
// observations, and rebuilds one immutable effective catalog from those
// layers. Reads reach no external system.
type Runtime struct {
	client *starmap.Client
	config options
	source Source

	// providerRetentionMu serializes observation selection and durable provider writes.
	providerRetentionMu sync.Mutex

	// publicationMu orders complete rebuilds and their effective-state activation.
	publicationMu sync.Mutex

	// mu guards the retained layers and the published effective state.
	mu                 sync.RWMutex
	layers             layerSet
	effective          starmap.CatalogState
	report             statusState
	permissions        authorityPermissions
	permissionRuns     runGroup
	permissionIO       sync.Mutex
	authorityObservers authorityObservationGroup

	instanceSeed string
	directory    *flock.Flock
	store        *layerStore
	lease        *leaseKeeper
	schedule     scheduler
	runs         runGroup

	// updatesMu guards the publication channel. Close marks the channel closed
	// under the lock that broadcast holds. A caller-owned run that publishes
	// after Close then sends nothing.
	updatesMu     sync.Mutex
	updatesClosed bool
	updates       chan starmap.CatalogState

	ctx       context.Context
	cancel    context.CancelFunc
	work      sync.WaitGroup
	closeOnce sync.Once
	closeErr  error
}

// Open returns a connected runtime. It serves the verified embedded catalog
// before the first upstream reply, so Catalog and State never wait for the
// network. Open starts the source and acquisition schedules and returns.
func Open(ctx context.Context, opts ...Option) (*Runtime, error) {
	if ctx == nil {
		return nil, &errors.ValidationError{Field: "context", Message: "is required"}
	}
	config, err := defaults().apply(opts...)
	if err != nil {
		return nil, err
	}
	config.resolve()
	if err := config.validate(); err != nil {
		return nil, err
	}
	config.sourceConfiguration, err = describeSources(config)
	if err != nil {
		return nil, err
	}
	directory, err := acquireDirectory(ctx, config.stateDirectory)
	if err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened && directory != nil {
			_ = directory.Close()
		}
	}()
	if err := refusePendingMigration(config.stateDirectory); err != nil {
		return nil, err
	}
	if err := refuseRetiredMigration(config.stateDirectory); err != nil {
		return nil, err
	}
	if config.publishedMigration != nil {
		if err := VerifyDirectoryMigrationPublication(ctx, *config.publishedMigration); err != nil {
			return nil, err
		}
	}
	if config.completedMigration != nil {
		if err := verifyCompletedMigrationSelection(ctx, *config.completedMigration); err != nil {
			return nil, err
		}
	}
	if err := bindDirectoryOwner(ctx, config.stateDirectory, config.directoryOwner, config.schedulerIdentity); err != nil {
		return nil, errors.WrapResource("bind", "runtime directory owner", "", err)
	}
	seed, err := prepareInstanceSeed(ctx, config.stateDirectory)
	if err != nil {
		return nil, errors.WrapResource("prepare", "runtime instance seed", "", err)
	}
	client, err := starmap.NewContext(ctx, config.acquisitionPolicyClientOptions()...)
	if err != nil {
		return nil, err
	}
	if err := repairWorkspaceForStartup(ctx, client); err != nil {
		return nil, err
	}

	runtime := &Runtime{
		client:       client,
		directory:    directory,
		instanceSeed: seed,
		config:       *config,
		updates:      make(chan starmap.CatalogState, updatesBuffer),
	}
	runtime.ctx, runtime.cancel = context.WithCancel(context.WithoutCancel(ctx))

	// Source selection is terminal. A configured custom source never falls
	// back to the public channel, so a selection failure fails Open.
	runtime.source, err = runtime.selectSource()
	if err != nil {
		runtime.cancel()
		return nil, err
	}

	runtime.store, err = newLayerStore(runtime.config.stateDirectory)
	if err != nil {
		runtime.cancel()
		return nil, err
	}
	if err := runtime.store.recoverInputPublication(ctx, client.CurrentCatalogState()); err != nil {
		runtime.cancel()
		return nil, err
	}
	if err := runtime.loadRetainedLayers(ctx); err != nil {
		runtime.cancel()
		return nil, err
	}
	if err := runtime.initializeEffective(ctx); err != nil {
		runtime.cancel()
		return nil, err
	}
	if err := runtime.initializeAuthority(); err != nil {
		runtime.cancel()
		return nil, err
	}
	if err := runtime.initializeSchedule(); err != nil {
		runtime.cancel()
		return nil, err
	}
	runtime.adoptSourceIdentity()
	runtime.lease = newLeaseKeeper(
		runtime.config.leaseStore,
		runtime.schedule.identity.Instance,
		runtime.config.now,
	)
	if err := runtime.lease.start(runtime.ctx, &runtime.work, runtime.onLeaseLost); err != nil {
		runtime.cancel()
		return nil, err
	}

	if err := runtime.publishAcquisitionPolicyStartup(ctx); err != nil {
		runtime.abort()
		return nil, errors.WrapResource("publish", "active binding catalog", "", err)
	}

	if err := runtime.startPermissionClock(); err != nil {
		runtime.abort()
		return nil, err
	}
	if err := runtime.prepareSourceStartup(ctx); err != nil {
		runtime.abort()
		return nil, err
	}
	runtime.startSchedules()
	opened = true
	return runtime, nil
}

// abort releases what a failed Open already started. It cancels runtime-owned
// work and returns the lease, so a failed Open leaves no holder behind.
func (r *Runtime) abort() {
	r.cancel()
	r.lease.stop()
}

// Catalog returns the current immutable effective catalog. It reaches no
// external system and never blocks on the source.
func (r *Runtime) Catalog() *catalogs.Catalog {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.effective.Catalog
}

// State returns one atomic snapshot of the effective catalog, generation identity, and authority head.
// It allocates no memory and reaches no external system. The authority head does not grant permission.
func (r *Runtime) State() starmap.CatalogState {
	if r == nil {
		return starmap.CatalogState{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.effective
}

// Client returns the immutable publication client underneath the runtime.
// Use it for explicit publication, hooks, and generation retrieval.
func (r *Runtime) Client() *starmap.Client {
	if r == nil {
		return nil
	}
	return r.client
}

// Updates returns the channel that carries every published effective catalog
// state. The runtime buffers the channel. A reader that falls behind loses
// intermediate states and always observes the newest one.
func (r *Runtime) Updates() <-chan starmap.CatalogState {
	if r == nil {
		return nil
	}
	return r.updates
}

// Close stops runtime-owned work and releases the lease. It is idempotent and
// joins within five seconds. A run that does not stop in time leaves a typed
// timeout error. Directory ownership remains held until work and lease release finish.
func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		r.authorityObservers.close()
		active := r.runs.close()
		permissionActive := r.permissionRuns.close()
		r.cancel()
		joined := make(chan error, 1)
		go func() {
			<-active
			<-permissionActive
			r.authorityObservers.active.Wait()
			r.work.Wait()
			r.lease.stop()
			var err error
			sealContext, sealCancel := context.WithTimeout(context.Background(), closeJoinTimeout)
			err = r.sealAuthority(sealContext)
			sealCancel()
			if r.directory != nil {
				err = stderrors.Join(err, r.directory.Close())
			}
			joined <- err
		}()
		timer := time.NewTimer(closeJoinTimeout)
		defer timer.Stop()
		select {
		case err := <-joined:
			r.closeErr = err
		case <-timer.C:
			r.closeErr = &errors.TimeoutError{
				Operation: "close starmap runtime",
				Duration:  closeJoinTimeout.String(),
			}
		}
		r.updatesMu.Lock()
		r.updatesClosed = true
		close(r.updates)
		r.updatesMu.Unlock()
	})
	return r.closeErr
}

// initializeEffective selects startup state and retains the separate compiled baseline.
// An explicit binding set always rebuilds, including when no retained evidence is active.
// Without that set, an empty layer set keeps only unscoped accepted state.
// It reaches no external system.
func (r *Runtime) initializeEffective(ctx context.Context) error {
	current := r.client.CurrentCatalogState()
	baseline := r.client.EmbeddedCatalogState()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.layers.embedded = baseline
	r.layers.requireAuthority = r.requiresAuthority()
	r.layers.providerBindings = r.config.providerBindings
	r.layers.acquisitionSources = r.config.acquisitionSources
	if !r.requiresAuthority() && r.layers.empty() && r.config.providerBindings == nil && r.config.acquisitionSources == nil {
		if storedProviderPolicyRequired(current) {
			return &errors.ConflictError{Resource: "catalog startup policy", Message: "stored scoped evidence requires explicit provider bindings or retained input recovery"}
		}
		r.effective = current
		r.report.startedAt = r.config.now()
		return nil
	}
	state, err := r.layers.build(ctx, baseline)
	if err != nil {
		return err
	}
	if err := current.Catalog.CanonicalAliases().ValidateSuccessor(state.Catalog.CanonicalAliases()); err != nil {
		return err
	}
	r.effective = state
	r.report.startedAt = r.config.now()
	return nil
}

// commitOrdinary durably publishes one effective catalog when the deployment holds a
// writable store. The epoch that the run started under fences the commit, so an
// instance that lost the lease cannot overwrite a newer generation.
func (r *Runtime) commitOrdinary(ctx context.Context, state starmap.CatalogState, epoch uint64, evidence starmap.CandidateEvidence, source *sourceLayer) (starmap.CatalogState, error) {
	if err := ctx.Err(); err != nil {
		return starmap.CatalogState{}, err
	}
	ctx = r.authorityPublicationContext(ctx)
	if r.requiresAuthority() {
		return r.commitAuthority(ctx, state, source, epoch)
	}
	if !r.client.PublishesDurably() {
		// Without a durable store the runtime publishes in memory only. The
		// effective catalog stays correct. It does not survive a restart.
		return state, nil
	}
	if restored, err := r.restoreAcquisitionPolicyGeneration(ctx, state, epoch); err != nil {
		return starmap.CatalogState{}, err
	} else if restored {
		return r.client.CurrentCatalogState(), nil
	}
	publication, err := r.client.Update(ctx, func(
		context.Context,
		*catalogs.Catalog,
	) (*starmap.Candidate, error) {
		if err := r.lease.fence(epoch); err != nil {
			return nil, err
		}
		if state.GenerationID == "" {
			// A baseline that names no identity leaves the client to mint one.
			return starmap.NewCandidate(state.Catalog, evidence)
		}
		// The retained layers decide the identity of the effective catalog. A
		// rebuild that derives the committed identity again also serves the
		// committed bytes, so it commits no second generation.
		if state.GenerationID == r.client.CurrentGenerationID() {
			return nil, nil
		}
		// The durable generation keeps the derived identity. An in-memory
		// runtime and a runtime with a catalog store then report one identity
		// for one set of layers, and a restart reports it again.
		return starmap.NewCandidate(
			state.Catalog,
			evidence,
			starmap.WithCandidateGenerationID(state.GenerationID),
		)
	})
	if err != nil {
		return starmap.CatalogState{}, err
	}
	if !publication.Published {
		return state, nil
	}
	return r.client.CurrentCatalogState(), nil
}

// broadcast delivers one published state to the updates channel. A full
// channel drops the oldest pending state, so the newest state always arrives.
// A closed runtime delivers nothing, because Close owns the channel.
func (r *Runtime) broadcast(state starmap.CatalogState) {
	r.updatesMu.Lock()
	defer r.updatesMu.Unlock()
	if r.updatesClosed {
		return
	}
	for {
		select {
		case r.updates <- state:
			return
		default:
		}
		select {
		case <-r.updates:
		default:
			return
		}
	}
}

// onLeaseLost cancels every runtime-owned run after the lease keeper loses the
// lease. The runtime keeps serving its retained catalog. It stops writing.
func (r *Runtime) onLeaseLost() {
	logging.Warn().
		Str("reason", string(leaseLost)).
		Msg("Runtime lost the refresh lease and cancelled its active run")
	r.runs.cancelActive()
}
