package starmap

import (
	"context"
	stderrors "errors"
	"strconv"
	"sync"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

// Lease timing. One holder renews well inside the expiry, so a short pause
// never drops the lease and a stopped holder expires quickly.
const (
	// LeaseTTL bounds how long one lease stays valid without a renewal.
	LeaseTTL = 90 * time.Second

	// LeaseRenewInterval is the period between renewals.
	LeaseRenewInterval = 30 * time.Second
)

// Lease is the exclusive right to commit a durable catalog generation. The
// epoch increases on every fresh acquisition, so a commit that carries an older
// epoch is stale.
type Lease struct {
	// Holder names the instance that owns the lease.
	Holder string

	// Epoch increases on every fresh acquisition.
	Epoch uint64

	// ExpiresAt is when the lease lapses without a renewal.
	ExpiresAt time.Time
}

// LeaseStore is the shared-storage lease that fences durable commits. A
// deployment whose instances share no storage needs no lease store, and the
// runtime then commits without a fence.
type LeaseStore interface {
	// AcquireLease takes the lease for the named holder. A store that another
	// holder owns returns an *errors.ConflictError. That refusal names a
	// non-owner state, not a failure. The runtime keeps serving its retained
	// catalog and tries again at the next run. Every other error fails Open.
	AcquireLease(ctx context.Context, holder string, ttl time.Duration) (Lease, error)

	// Renew extends a held lease. It fails when another holder took the lease.
	Renew(ctx context.Context, lease Lease, ttl time.Duration) (Lease, error)

	// Release returns the lease early.
	Release(ctx context.Context, lease Lease) error
}

// leaseState names what the runtime knows about its lease.
type leaseState string

const (
	// leaseNotRequired means the deployment shares no storage.
	leaseNotRequired leaseState = "lease_not_required"

	// leaseHeld means this instance owns the lease.
	leaseHeld leaseState = "lease_held"

	// leaseLost means another instance took the lease.
	leaseLost leaseState = "lease_lost"
)

// leaseKeeper takes the runtime lease, renews it, and reports its loss. It
// fences every durable commit with the epoch of the acquisition that the
// commit started under.
type leaseKeeper struct {
	store  LeaseStore
	holder string
	now    func() time.Time

	// base is the runtime context. Renewal runs under it, so Close stops
	// renewal even when a later run took the lease again.
	base   context.Context
	work   *sync.WaitGroup
	onLost func()

	mu      sync.RWMutex
	lease   Lease
	state   leaseState
	stopped bool

	stopOnce    sync.Once
	cancelRenew context.CancelFunc
}

// newLeaseKeeper returns the lease keeper for one runtime. A nil store selects
// the unfenced path, which suits a deployment with no shared storage.
func newLeaseKeeper(store LeaseStore, holder string, now func() time.Time) *leaseKeeper {
	state := leaseNotRequired
	if store != nil {
		state = leaseLost
	}
	return &leaseKeeper{store: store, holder: holder, now: now, state: state}
}

// start takes the lease and renews it until the context ends. Another holder
// makes this instance a consumer of accepted state, not a failure: the keeper
// records lease_lost and Open returns a usable runtime.
func (k *leaseKeeper) start(ctx context.Context, work *sync.WaitGroup, onLost func()) error {
	if k.store == nil {
		return nil
	}
	k.base = ctx
	k.work = work
	k.onLost = onLost
	err := k.take(ctx)
	if isLeaseRefusal(err) {
		logging.Info().
			Str("holder", k.holder).
			Msg("Another instance owns the refresh lease; this instance consumes accepted state")
		return nil
	}
	return err
}

// ensureHeld takes the lease again before one run starts. A non-owner that
// stays refused sends no upstream request and observes no provider.
func (k *leaseKeeper) ensureHeld(ctx context.Context) error {
	if k == nil || k.store == nil {
		return nil
	}
	k.mu.RLock()
	state := k.state
	stopped := k.stopped
	k.mu.RUnlock()
	if stopped {
		return &errors.ConflictError{
			Resource: "runtime lease",
			Expected: string(leaseHeld),
			Actual:   string(leaseLost),
			Message:  "the runtime closed and released the lease",
		}
	}
	if state == leaseHeld {
		return nil
	}
	return k.take(ctx)
}

// take takes the lease and restarts renewal. A refusal leaves the keeper in
// the lost state and returns the typed conflict.
func (k *leaseKeeper) take(ctx context.Context) error {
	lease, err := k.store.AcquireLease(ctx, k.holder, LeaseTTL)
	if err != nil {
		k.mu.Lock()
		k.state = leaseLost
		k.mu.Unlock()
		return errors.WrapResource("acquire", "runtime lease", k.holder, err)
	}

	k.mu.Lock()
	if k.stopped {
		k.mu.Unlock()
		return &errors.ConflictError{
			Resource: "runtime lease",
			Expected: string(leaseHeld),
			Actual:   string(leaseLost),
			Message:  "the runtime closed and released the lease",
		}
	}
	k.lease = lease
	k.state = leaseHeld
	previous := k.cancelRenew
	renewCtx, cancel := context.WithCancel(k.base)
	k.cancelRenew = cancel
	k.mu.Unlock()

	// One renewal loop runs at a time. The previous loop already returned
	// after it reported the loss, and this cancellation covers every other
	// order of events.
	if previous != nil {
		previous()
	}
	k.work.Go(func() {
		k.renewUntil(renewCtx, k.onLost)
	})
	return nil
}

// isLeaseRefusal reports whether another holder owns the lease. Every other
// failure is a store failure, and Open reports it.
func isLeaseRefusal(err error) bool {
	if err == nil {
		return false
	}
	var conflict *errors.ConflictError
	return stderrors.As(err, &conflict)
}

// renewUntil renews the lease on every interval. It reports one loss and
// returns, so the runtime cancels its active run within one renewal interval.
func (k *leaseKeeper) renewUntil(ctx context.Context, onLost func()) {
	timer := time.NewTimer(LeaseRenewInterval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := k.renewOnce(ctx); err != nil {
				if onLost != nil {
					onLost()
				}
				return
			}
			timer.Reset(LeaseRenewInterval)
		}
	}
}

// renewOnce extends the held lease. A failure marks the lease lost.
func (k *leaseKeeper) renewOnce(ctx context.Context) error {
	k.mu.RLock()
	current := k.lease
	k.mu.RUnlock()

	renewed, err := k.store.Renew(ctx, current, LeaseTTL)
	if err != nil {
		k.mu.Lock()
		k.state = leaseLost
		k.mu.Unlock()
		logging.Warn().
			Err(err).
			Str("holder", k.holder).
			Msg("Runtime lease renewal failed")
		return errors.WrapResource("renew", "runtime lease", k.holder, err)
	}
	k.mu.Lock()
	k.lease = renewed
	k.state = leaseHeld
	k.mu.Unlock()
	return nil
}

// epoch returns the epoch a commit must carry. It returns zero when the
// deployment needs no lease.
func (k *leaseKeeper) epoch() uint64 {
	if k == nil || k.store == nil {
		return 0
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.lease.Epoch
}

// fence rejects a durable commit that started under an older lease epoch, or
// that runs after this instance lost the lease.
func (k *leaseKeeper) fence(epoch uint64) error {
	if k == nil || k.store == nil {
		return nil
	}
	k.mu.RLock()
	state := k.state
	current := k.lease
	k.mu.RUnlock()

	if state != leaseHeld {
		return &errors.ConflictError{
			Resource: "runtime lease",
			Expected: string(leaseHeld),
			Actual:   string(state),
			Message:  "the instance no longer holds the runtime lease",
		}
	}
	if current.Epoch != epoch {
		return &errors.ConflictError{
			Resource: "runtime lease",
			Expected: strconv.FormatUint(epoch, 10),
			Actual:   strconv.FormatUint(current.Epoch, 10),
			Message:  "the commit started under an older lease epoch",
		}
	}
	if !current.ExpiresAt.IsZero() && !k.now().Before(current.ExpiresAt) {
		return &errors.ConflictError{
			Resource: "runtime lease",
			Expected: string(leaseHeld),
			Actual:   string(leaseLost),
			Message:  "the runtime lease expired before the commit",
		}
	}
	return nil
}

// status reports the current lease state.
func (k *leaseKeeper) status() leaseState {
	if k == nil || k.store == nil {
		return leaseNotRequired
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.state
}

// stop ends renewal and releases the lease. It is idempotent.
func (k *leaseKeeper) stop() {
	if k == nil || k.store == nil {
		return
	}
	k.stopOnce.Do(func() {
		k.mu.Lock()
		k.stopped = true
		cancel := k.cancelRenew
		current := k.lease
		state := k.state
		k.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		if state != leaseHeld {
			return
		}
		releaseCtx, cancel := context.WithTimeout(context.Background(), LeaseRenewInterval)
		defer cancel()
		if err := k.store.Release(releaseCtx, current); err != nil {
			logging.Warn().
				Err(err).
				Str("holder", k.holder).
				Msg("Runtime lease release failed; the lease expires on its own")
		}
	})
}
