package runtime

import "sync"

// authorityObservationGroup tracks source callbacks until shutdown joins them.
type authorityObservationGroup struct {
	mu     sync.Mutex
	active sync.WaitGroup
	closed bool
}

func (g *authorityObservationGroup) begin() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return false
	}
	g.active.Add(1)
	return true
}

func (g *authorityObservationGroup) close() {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
}
