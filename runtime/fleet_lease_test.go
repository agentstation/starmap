package runtime

import (
	"context"
	"testing"
	"time"
)

type fleetRenewalStore struct {
	LeaseStore
	renew func(Lease) (Lease, error)
}

func (s *fleetRenewalStore) Renew(_ context.Context, lease Lease, _ time.Duration) (Lease, error) {
	return s.renew(lease)
}

func TestFleetLeaseRenewalCannotReplaceGrant(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		change func(*Lease)
	}{
		{"holder", func(l *Lease) { l.Holder = "other" }},
		{"epoch", func(l *Lease) { l.Epoch++ }},
		{"session", func(l *Lease) { l.SessionID = "other" }},
		{"backend", func(l *Lease) { l.Identity.BackendID = "other" }},
		{"recovery-epoch", func(l *Lease) { l.Identity.RecoveryEpoch++ }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			original := fleetTestPublication(t).Grant
			store := &fleetRenewalStore{renew: func(lease Lease) (Lease, error) { scenario.change(&lease); return lease, nil }}
			keeper := newLeaseKeeper(store, original.Holder, time.Now)
			keeper.lease, keeper.state = original, leaseHeld
			if err := keeper.renewOnce(t.Context()); err == nil {
				t.Fatal("renewal substituted different ownership evidence")
			}
			if keeper.lease != original || keeper.status() != leaseLost {
				t.Fatal("renewal installed a different grant")
			}
		})
	}
}

func TestFleetLeaseLateRenewalCannotReplaceCurrentState(t *testing.T) {
	for _, scenario := range []string{"closed", "lost", "new-grant", "new-grant-error"} {
		t.Run(scenario, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			store := &fleetRenewalStore{renew: func(lease Lease) (Lease, error) {
				close(entered)
				<-release
				if scenario == "new-grant-error" {
					return Lease{}, fleetConflict("the previous grant expired")
				}
				lease.ExpiresAt = time.Now().Add(LeaseTTL)
				return lease, nil
			}}
			original := fleetTestPublication(t).Grant
			keeper := newLeaseKeeper(store, original.Holder, time.Now)
			keeper.lease, keeper.state = original, leaseHeld
			finished := make(chan error, 1)
			go func() { finished <- keeper.renewOnce(t.Context()) }()
			<-entered
			keeper.mu.Lock()
			if scenario == "closed" {
				keeper.stopped = true
				keeper.state = leaseLost
			} else if scenario == "lost" {
				keeper.state = leaseLost
			} else {
				keeper.lease.Epoch++
			}
			expected, state := keeper.lease, keeper.state
			keeper.mu.Unlock()
			close(release)
			if err := <-finished; err == nil {
				t.Fatal("late renewal reported current ownership")
			}
			if keeper.lease != expected || keeper.status() != state {
				t.Fatal("late renewal replaced newer local ownership state")
			}
		})
	}
}
