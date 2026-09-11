package permission

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agentstation/starmap/pkg/errors"
)

// ClockMonitor owns explicit host refresh work over one qualified clock cache.
// Construction starts no observation or worker. The host calls Start and Close.
// Read uses cached evidence. Source failures leave diagnostics available.
type ClockMonitor struct {
	cache    *ClockCache
	interval time.Duration
	run      atomic.Pointer[clockMonitorRun]
	observed atomic.Pointer[clockMonitorObservation]

	lifecycle sync.Mutex
	closed    bool
}

type clockMonitorRun struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

type clockMonitorObservation struct {
	attempts uint64
	err      error
}

// ClockMonitorStatus describes local observation work and current cached validity.
// Known can expire between status inspection and admission. Admission must use Read.
type ClockMonitorStatus struct {
	// Running reports whether the host runs work with an active context.
	Running bool
	// Known reports whether the cached sample still meets its age and error bounds.
	Known bool
	// Attempts counts completed observations, including failed observations.
	Attempts uint64
	// LastError is the last observation failure. A successful observation clears it.
	LastError error
}

// NewClockMonitor validates a host-selected source and schedule without I/O.
// The interval must be positive and less than half the maximum sample age.
// Query delay consumes the sample's age bound even with this scheduling headroom.
func NewClockMonitor(config ClockCacheConfig, interval time.Duration) (*ClockMonitor, error) {
	cache, err := NewClockCache(config)
	if err != nil {
		return nil, err
	}
	if interval <= 0 || interval >= config.MaxAge/2 {
		return nil, clockCacheError("refresh interval must be positive and below half the maximum sample age")
	}
	return &ClockMonitor{cache: cache, interval: interval}, nil
}

// Start starts one observation worker and returns without waiting for a source.
// An unavailable source leaves Read unqualified while the worker retries on schedule.
// A monitor starts at most once. Reconstruct it after shutdown or parent cancellation.
func (m *ClockMonitor) Start(ctx context.Context) error {
	if m == nil || m.cache == nil || ctx == nil {
		return clockCacheError("a constructed clock monitor and context are required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.lifecycle.Lock()
	defer m.lifecycle.Unlock()
	if m.closed || m.run.Load() != nil {
		return &errors.ConflictError{Resource: "permission clock", Message: "monitor already started or closed"}
	}
	ctx, cancel := context.WithCancel(ctx)
	run := &clockMonitorRun{ctx: ctx, cancel: cancel, done: make(chan struct{})}
	m.run.Store(run)
	go m.observe(run)
	return nil
}

func (m *ClockMonitor) observe(run *clockMonitorRun) {
	defer close(run.done)
	defer m.cache.Invalidate()
	timer := time.NewTimer(m.interval)
	defer timer.Stop()
	var attempts uint64
	for run.ctx.Err() == nil {
		err := m.cache.Refresh(run.ctx)
		attempts++
		m.observed.Store(&clockMonitorObservation{attempts: attempts, err: err})
		// Start the next interval after this observation. Queries never overlap.
		timer.Reset(m.interval)
		select {
		case <-run.ctx.Done():
			return
		case <-timer.C:
		}
	}
}

// Read returns qualified cached time only while the owning host context is active.
// Successful cached reads use local memory. They allocate no memory and take no lifecycle lock.
func (m *ClockMonitor) Read() ClockReading {
	if m == nil || m.cache == nil {
		return ClockReading{}
	}
	run := m.run.Load()
	if run == nil || run.ctx.Err() != nil {
		return ClockReading{}
	}
	return m.cache.Read()
}

// Status returns diagnostics without a time-service query.
// A stopped monitor retains its last observation result but reports no valid sample.
func (m *ClockMonitor) Status() ClockMonitorStatus {
	if m == nil {
		return ClockMonitorStatus{}
	}
	status := ClockMonitorStatus{Known: m.Read().Known}
	if run := m.run.Load(); run != nil {
		status.Running = run.ctx.Err() == nil
	}
	if !status.Running {
		status.Known = false
	}
	if observation := m.observed.Load(); observation != nil {
		status.Attempts = observation.attempts
		status.LastError = observation.err
	}
	return status
}

// Close cancels observations, invalidates cached evidence, and waits for cleanup.
// It is safe before Start and for concurrent or repeated calls.
// The configured observation source must honor its context, as ClockCacheConfig requires.
func (m *ClockMonitor) Close() {
	if m == nil || m.cache == nil {
		return
	}
	m.lifecycle.Lock()
	m.closed = true
	run := m.run.Load()
	if run != nil {
		run.cancel()
	}
	m.cache.Invalidate()
	m.lifecycle.Unlock()
	if run != nil {
		<-run.done
	}
}
