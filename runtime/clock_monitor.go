package runtime

import (
	"github.com/agentstation/starmap/pkg/catalogs/permission"
	"github.com/agentstation/starmap/pkg/errors"
)

// WithoutPermissionClock clears earlier monitor and callback selections.
// No clock evidence remains until a later option supplies a clock explicitly.
func WithoutPermissionClock() Option {
	return func(o *options) error {
		o.permissionClockMonitor = nil
		o.permissionClockReading = nil
		o.permissionClockUncertainty = nil
		return nil
	}
}

// WithPermissionClockMonitor assigns one unstarted monitor to this runtime.
// Open starts it before source startup and owns its shutdown after a successful start.
// A failed Open cancels any monitor it started. It leaves a monitor owned elsewhere alone.
// Select this option without either external permission-clock callback.
// An origin can also use this monitor's Read method in OriginConfig.Clock.
func WithPermissionClockMonitor(monitor *permission.ClockMonitor) Option {
	return func(o *options) error {
		if monitor == nil {
			return &errors.ValidationError{Field: "permission_clock", Message: "a monitor is required"}
		}
		o.permissionClockMonitor = monitor
		return nil
	}
}

func (r *Runtime) startPermissionClock() error {
	monitor := r.config.permissionClockMonitor
	if monitor == nil {
		return nil
	}
	if err := monitor.Start(r.ctx); err != nil {
		return errors.WrapResource("start", "permission clock monitor", "", err)
	}
	// Join the monitor inside the runtime's existing bounded shutdown.
	r.work.Go(func() {
		<-r.ctx.Done()
		monitor.Close()
	})
	return nil
}

// PermissionClockStatus reports diagnostics for the runtime-owned clock monitor.
// It starts no observation. A runtime with an external callback returns an empty status.
// LastError is a local diagnostic. Hosts must redact it in public responses.
func (r *Runtime) PermissionClockStatus() permission.ClockMonitorStatus {
	if r == nil || r.config.permissionClockMonitor == nil {
		return permission.ClockMonitorStatus{}
	}
	return r.config.permissionClockMonitor.Status()
}
