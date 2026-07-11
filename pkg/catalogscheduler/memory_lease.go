package catalogscheduler

import (
	"context"
	"sync"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

type memoryLeaseState struct {
	owner     string
	token     uint64
	expiresAt time.Time
}

// MemoryLease is a process-local reference lease. Sharing one instance across
// runner replicas models an external atomic lease service in deterministic tests.
type MemoryLease struct {
	mu     sync.Mutex
	leases map[string]memoryLeaseState
	next   uint64
	now    func() time.Time
}

// NewMemoryLease creates an empty reference lease service.
func NewMemoryLease() *MemoryLease {
	return &MemoryLease{leases: make(map[string]memoryLeaseState), now: time.Now}
}

// Acquire atomically acquires an absent or expired lease.
func (l *MemoryLease) Acquire(ctx context.Context, request LeaseRequest) (LeaseGuard, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now().UTC()
	if current, found := l.leases[request.Key]; found && current.expiresAt.After(now) {
		return nil, &errors.ConflictError{
			Resource: "catalog scheduler lease", Expected: request.Owner, Actual: current.owner,
			Message: "publisher lease is already held",
		}
	}
	l.next++
	state := memoryLeaseState{owner: request.Owner, token: l.next, expiresAt: now.Add(request.TTL)}
	l.leases[request.Key] = state
	return &memoryLeaseGuard{lease: l, key: request.Key, owner: request.Owner, token: state.token}, nil
}

type memoryLeaseGuard struct {
	once  sync.Once
	lease *MemoryLease
	key   string
	owner string
	token uint64
	err   error
}

func (g *memoryLeaseGuard) Release(ctx context.Context) error {
	g.once.Do(func() {
		if err := ctx.Err(); err != nil {
			g.err = err
			return
		}
		g.lease.mu.Lock()
		defer g.lease.mu.Unlock()
		current, found := g.lease.leases[g.key]
		if found && current.owner == g.owner && current.token == g.token {
			delete(g.lease.leases, g.key)
		}
	})
	return g.err
}
