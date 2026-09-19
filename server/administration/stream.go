package administration

import (
	"context"
	"time"
)

// Track cancels a long-lived request when its credential expires or loses permission.
// The caller must call the returned cancel function when the request ends.
func (m *Manager) Track(ctx context.Context, principal Principal) (context.Context, context.CancelFunc) {
	tracked, cancel := context.WithCancel(ctx)
	changed, expiry, valid := m.credentialWatch(principal)
	if !valid {
		cancel()
		return tracked, cancel
	}
	go func() {
		defer cancel()
		for {
			var timer *time.Timer
			var deadline <-chan time.Time
			if !expiry.IsZero() {
				timer = time.NewTimer(expiry.Sub(m.clock()))
				deadline = timer.C
			}
			select {
			case <-tracked.Done():
				if timer != nil {
					timer.Stop()
				}
				return
			case <-changed:
			case <-deadline:
			}
			if timer != nil {
				timer.Stop()
			}
			changed, expiry, valid = m.credentialWatch(principal)
			if !valid {
				return
			}
		}
	}()
	return tracked, cancel
}

func (m *Manager) credentialWatch(principal Principal) (<-chan struct{}, time.Time, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := m.snapshot.Load()
	if snapshot == nil || principal.audience != m.audience {
		return nil, time.Time{}, false
	}
	current, accepted := snapshot.authenticateDigest(principal.key, m.clock())
	if !accepted || current != principal {
		return nil, time.Time{}, false
	}
	return m.changed, snapshot.keys[principal.key].notAfter, true
}

func (m *Manager) notifyCredentials() {
	close(m.changed)
	m.changed = make(chan struct{})
}
