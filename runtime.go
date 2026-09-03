package starmap

import (
	"context"
	"sync"
	"time"

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

// Source reads one upstream immutable catalog generation. Every built-in
// source verifies its evidence before it returns a generation.
type Source interface {
	// Identity returns the safe identity of the source. It names no URL, no
	// host, and no credential.
	Identity() string

	// Read returns the current upstream generation. A source that finds no
	// change reports Changed false and carries no generation.
	Read(ctx context.Context) (SourceRead, error)
}

// SourceWatcher is an optional Source that reports an upstream change as it
// arrives. A reactive source, such as one Starmap cascaded onto another,
// learns of a publication on its own stream. The runtime then refreshes on
// that wake and waits for no poll boundary. A delta crosses a cascade in
// seconds instead of in one poll interval.
//
// The channel carries an empty value for each change and holds at most one
// pending wake. A closed channel means the source reports no further change,
// and the runtime falls back to its poll interval.
type SourceWatcher interface {
	Source

	// Changes reports each upstream change as one wake.
	Changes() <-chan struct{}
}

// SourceIdentityAdopter is an optional Source that takes the fleet instance
// identity of its runtime. The source and the runtime then spread their work
// on one identity, so a replica keeps one stable phase for every controller it
// owns. Open hands the identity over before the first read.
type SourceIdentityAdopter interface {
	Source

	// AdoptInstanceIdentity takes the derived instance identity of the runtime.
	AdoptInstanceIdentity(instance string)
}

// SourceHop is one sanitized entry in an upstream source chain. A hop names
// the reporting identity and its health, never an address.
type SourceHop struct {
	Identity    string
	Health      Health
	PublishedAt time.Time
	ObservedAt  time.Time
}

// SourceRead is one upstream observation.
type SourceRead struct {
	// Changed reports whether the upstream generation moved.
	Changed bool

	// Generation is the verified immutable catalog generation. It is empty
	// when Changed is false.
	Generation catalogs.Generation

	// PublishedAt is the upstream publication time.
	PublishedAt time.Time

	// ChannelUpdatedAt is when the upstream channel last moved.
	ChannelUpdatedAt time.Time

	// Chain is the sanitized upstream source chain, nearest hop first.
	Chain []SourceHop

	// Health is the upstream-reported health of the source itself.
	Health Health
}

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
	// Eligible is the number of providers the run considered.
	Eligible int

	// Attempts holds one terminal attempt per eligible provider.
	Attempts []sources.ProviderAttempt

	// Layers holds one observation per provider that answered.
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
	client *Client
	config runtimeOptions
	source Source

	// mu guards the retained layers and the published effective state.
	mu        sync.RWMutex
	layers    layerSet
	effective CatalogState
	report    statusState

	store    *layerStore
	lease    *leaseKeeper
	schedule scheduler
	runs     runGroup

	// updatesMu guards the publication channel. Close marks the channel closed
	// under the lock that broadcast holds. A caller-owned run that publishes
	// after Close then sends nothing.
	updatesMu     sync.Mutex
	updatesClosed bool
	updates       chan CatalogState

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
	config.runtime.resolve()
	if err := config.runtime.validate(); err != nil {
		return nil, err
	}
	client, err := newClient(ctx, config)
	if err != nil {
		return nil, err
	}

	runtime := &Runtime{
		client:  client,
		config:  config.runtime,
		updates: make(chan CatalogState, updatesBuffer),
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
	if err := runtime.loadRetainedLayers(); err != nil {
		runtime.cancel()
		return nil, err
	}
	if err := runtime.initializeEffective(); err != nil {
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

	// The require_source policy blocks inside the Open context and reads the
	// source once. A failed read fails Open, so a deployment that needs
	// upstream state never serves the embedded baseline instead. A non-owner
	// replica reads nothing, because the lease owner supplies the state that
	// this replica then consumes.
	if runtime.config.source.StartupPolicy == StartupRequireSource {
		if runtime.lease.status() == leaseLost {
			logging.Info().
				Str("holder", runtime.schedule.identity.Instance).
				Msg("The lease owner supplies the source state; require_source reads nothing here")
		} else if _, err := runtime.RefreshSource(ctx); err != nil {
			runtime.abort()
			return nil, errors.WrapResource(
				"read", "catalog source", runtime.source.Identity(), err)
		}
	}
	runtime.startSchedules()
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

// State returns one atomic snapshot of the effective catalog and its
// generation identity. It reaches no external system.
func (r *Runtime) State() CatalogState {
	if r == nil {
		return CatalogState{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.effective
}

// Client returns the immutable publication client underneath the runtime.
// Use it for explicit publication, hooks, and generation retrieval.
func (r *Runtime) Client() *Client {
	if r == nil {
		return nil
	}
	return r.client
}

// Updates returns the channel that carries every published effective catalog
// state. The runtime buffers the channel. A reader that falls behind loses
// intermediate states and always observes the newest one.
func (r *Runtime) Updates() <-chan CatalogState {
	if r == nil {
		return nil
	}
	return r.updates
}

// Close stops runtime-owned work and releases the lease. It is idempotent and
// joins within five seconds. A run that does not stop in time leaves a typed
// timeout error, so an operator sees the stall rather than a silent hang.
func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		r.cancel()
		r.runs.cancelActive()
		joined := make(chan struct{})
		go func() {
			r.work.Wait()
			close(joined)
		}()
		timer := time.NewTimer(closeJoinTimeout)
		defer timer.Stop()
		select {
		case <-joined:
		case <-timer.C:
			r.closeErr = &errors.TimeoutError{
				Operation: "close starmap runtime",
				Duration:  closeJoinTimeout.String(),
			}
		}
		r.lease.stop()
		r.updatesMu.Lock()
		r.updatesClosed = true
		close(r.updates)
		r.updatesMu.Unlock()
	})
	return r.closeErr
}

// initializeEffective publishes the starting effective catalog. It uses the
// retained layers when a previous run left any, and the client baseline
// otherwise. It reaches no external system.
func (r *Runtime) initializeEffective() error {
	baseline := r.client.CurrentCatalogState()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.layers.embedded = baseline
	if r.layers.empty() {
		r.effective = baseline
		r.report.startedAt = r.config.now()
		return nil
	}
	state, err := r.layers.build(baseline)
	if err != nil {
		return err
	}
	r.effective = state
	r.report.startedAt = r.config.now()
	return nil
}

// rebuild publishes a new effective catalog from the retained layers. The
// caller holds no lock. The rebuild runs under the runtime write lock, so a
// concurrent read never observes a partial generation. The epoch is the lease
// epoch of the run, so a run that lost the lease commits nothing.
func (r *Runtime) rebuild(ctx context.Context, epoch uint64) (CatalogState, error) {
	r.mu.Lock()
	baseline := r.layers.embedded
	state, err := r.layers.build(baseline)
	if err != nil {
		r.mu.Unlock()
		return CatalogState{}, err
	}
	r.mu.Unlock()

	durable, err := r.commit(ctx, state, epoch)
	if err != nil {
		return CatalogState{}, err
	}

	r.mu.Lock()
	r.effective = durable
	r.mu.Unlock()
	r.broadcast(durable)
	return durable, nil
}

// commit durably publishes one effective catalog when the deployment holds a
// writable store. The epoch that the run started under fences the commit, so an
// instance that lost the lease cannot overwrite a newer generation.
func (r *Runtime) commit(ctx context.Context, state CatalogState, epoch uint64) (CatalogState, error) {
	if r.client.requireWritableCatalogStore() != nil {
		// Without a durable store the runtime publishes in memory only. The
		// effective catalog stays correct. It does not survive a restart.
		return state, nil
	}
	publication, err := r.client.Update(ctx, func(
		context.Context,
		*catalogs.Catalog,
	) (*Candidate, error) {
		if err := r.lease.fence(epoch); err != nil {
			return nil, err
		}
		return NewCandidate(state.Catalog, CandidateEvidence{})
	})
	if err != nil {
		return CatalogState{}, err
	}
	if !publication.Published {
		return state, nil
	}
	return r.client.CurrentCatalogState(), nil
}

// broadcast delivers one published state to the updates channel. A full
// channel drops the oldest pending state, so the newest state always arrives.
// A closed runtime delivers nothing, because Close owns the channel.
func (r *Runtime) broadcast(state CatalogState) {
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
