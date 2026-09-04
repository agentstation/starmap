package operations

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"slices"
	"sync"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// DefaultRetained is the operation history that one registry keeps. A status
// read finds a recent operation, and the memory stays bounded.
const DefaultRetained = 64

// DefaultTimeout bounds one operation run. The registry cancels a run that
// passes the bound, and the status then reports the timeout reason.
const DefaultTimeout = 30 * time.Minute

// identityBytes is the operation identity width. A caller cannot guess an
// identity, so an operation status stays with its own client.
const identityBytes = 16

// Run runs one operation. It returns a bounded detail summary that the
// registry serializes into the status.
type Run func(ctx context.Context) (map[string]any, error)

// Option configures a registry.
type Option func(*Registry)

// WithRetained sets the operation history that the registry keeps.
func WithRetained(retained int) Option {
	return func(r *Registry) {
		if retained > 0 {
			r.retained = retained
		}
	}
}

// WithTimeout sets the bound on one operation run.
func WithTimeout(timeout time.Duration) Option {
	return func(r *Registry) {
		if timeout > 0 {
			r.timeout = timeout
		}
	}
}

// WithClock injects the registry clock. A test then reads exact timestamps
// without a real wait.
func WithClock(clock func() time.Time) Option {
	return func(r *Registry) {
		if clock != nil {
			r.clock = clock
		}
	}
}

type entry struct {
	status   Status
	cancel   context.CancelFunc
	done     chan struct{}
	canceled bool
}

// Registry starts, tracks, and cancels asynchronous operations. Every method is
// safe for concurrent use.
type Registry struct {
	mu       sync.Mutex
	entries  map[string]*entry
	order    []string
	totals   map[Sample]int
	closed   bool
	retained int
	timeout  time.Duration
	clock    func() time.Time
	group    sync.WaitGroup
}

// NewRegistry returns a registry that runs operations in the background.
func NewRegistry(options ...Option) *Registry {
	registry := &Registry{
		entries:  make(map[string]*entry),
		totals:   make(map[Sample]int),
		retained: DefaultRetained,
		timeout:  DefaultTimeout,
		clock:    time.Now,
	}
	for _, option := range options {
		option(registry)
	}
	return registry
}

// Start accepts one operation and runs it in the background. It returns the
// accepted status, so the caller reports an operation identity at once.
func (r *Registry) Start(kind Kind, run Run) (Status, error) {
	if !kind.Valid() {
		return Status{}, &errors.ValidationError{
			Field:   "operation.kind",
			Value:   string(kind),
			Message: "is not a known operation kind",
		}
	}
	if run == nil {
		return Status{}, &errors.ValidationError{
			Field: "operation.run", Message: "is required",
		}
	}
	id, err := newIdentity()
	if err != nil {
		return Status{}, err
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return Status{}, &errors.ConfigError{
			Component: "operation registry",
			Message:   "the registry no longer accepts operations",
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	item := &entry{
		status: Status{
			ID:         id,
			Kind:       kind,
			State:      StateAccepted,
			AcceptedAt: r.clock(),
		},
		cancel: cancel,
		done:   make(chan struct{}),
	}
	r.entries[id] = item
	r.order = append(r.order, id)
	r.countLocked(kind, StateAccepted, "")
	r.evictLocked()
	status := item.status.Copy()
	r.mu.Unlock()

	r.group.Go(func() {
		defer cancel()
		r.execute(ctx, item, run)
	})
	return status, nil
}

// Status returns the current snapshot of one operation. The second result
// reports whether the registry still holds the operation.
func (r *Registry) Status(id string) (Status, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, found := r.entries[id]
	if !found {
		return Status{}, false
	}
	return item.status.Copy(), true
}

// Cancel asks one operation to stop. It returns the snapshot at the request,
// and the second result reports whether the registry holds the operation.
func (r *Registry) Cancel(id string) (Status, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, found := r.entries[id]
	if !found {
		return Status{}, false
	}
	if !item.status.State.Terminal() {
		item.canceled = true
		item.cancel()
	}
	return item.status.Copy(), true
}

// Done returns a channel that closes when the operation reaches a terminal
// state. It returns nil for an operation that the registry does not hold.
func (r *Registry) Done(id string) <-chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, found := r.entries[id]
	if !found {
		return nil
	}
	return item.done
}

// Metrics returns one monotonic total for each kind, state, and reason. Every
// label comes from a closed set, so the metric cardinality stays bounded.
func (r *Registry) Metrics() []Sample {
	r.mu.Lock()
	samples := make([]Sample, 0, len(r.totals))
	for sample, total := range r.totals {
		sample.Total = total
		samples = append(samples, sample)
	}
	r.mu.Unlock()

	slices.SortFunc(samples, func(left, right Sample) int {
		if order := cmp.Compare(left.Kind, right.Kind); order != 0 {
			return order
		}
		if order := cmp.Compare(left.State, right.State); order != 0 {
			return order
		}
		return cmp.Compare(left.Reason, right.Reason)
	})
	return samples
}

// Close cancels every live operation and waits for the background runs. It
// returns a timeout error when a run does not join before the context ends.
func (r *Registry) Close(ctx context.Context) error {
	if ctx == nil {
		return &errors.ValidationError{Field: "context", Message: "is required"}
	}
	r.mu.Lock()
	r.closed = true
	for _, item := range r.entries {
		if !item.status.State.Terminal() {
			item.canceled = true
			item.cancel()
		}
	}
	r.mu.Unlock()

	joined := make(chan struct{})
	go func() {
		r.group.Wait()
		close(joined)
	}()
	select {
	case <-joined:
		return nil
	case <-ctx.Done():
		return &errors.TimeoutError{
			Operation: "operation registry shutdown",
			Message:   "a background operation did not join",
		}
	}
}

// execute runs one operation and records both transitions.
func (r *Registry) execute(ctx context.Context, item *entry, run Run) {
	r.mu.Lock()
	item.status.State = StateRunning
	item.status.StartedAt = r.clock()
	r.countLocked(item.status.Kind, StateRunning, "")
	r.mu.Unlock()

	detail, err := run(ctx)

	r.mu.Lock()
	item.status.CompletedAt = r.clock()
	item.status.Detail = detail
	switch {
	case err == nil:
		item.status.State = StateSucceeded
	case item.canceled:
		// A caller asked this operation to stop, so the error only reports the
		// stop. The status reports the cancellation instead of a failure.
		item.status.State = StateCanceled
	default:
		item.status.State = StateFailed
		// Sanitization happens here, before any log line and before the status
		// reaches a client. Only a closed-set reason code leaves this method.
		item.status.Reason = sources.ClassifyProviderReason(err)
	}
	r.countLocked(item.status.Kind, item.status.State, item.status.Reason)
	r.evictLocked()
	r.mu.Unlock()
	close(item.done)
}

// countLocked records one monotonic state entry. The caller holds the lock.
func (r *Registry) countLocked(kind Kind, state State, reason sources.ProviderReason) {
	r.totals[Sample{Kind: kind, State: state, Reason: reason}]++
}

// evictLocked drops the oldest terminal operations above the retained history.
// It stops at a live operation, so a running status stays readable.
func (r *Registry) evictLocked() {
	for len(r.order) > r.retained {
		id := r.order[0]
		item, found := r.entries[id]
		if found && !item.status.State.Terminal() {
			return
		}
		r.order = r.order[1:]
		delete(r.entries, id)
	}
}

// newIdentity returns one unguessable operation identity.
func newIdentity() (string, error) {
	buffer := make([]byte, identityBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", errors.WrapResource("generate", "operation identity", "", err)
	}
	return hex.EncodeToString(buffer), nil
}
