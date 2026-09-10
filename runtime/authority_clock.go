package runtime

import (
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/permission"
	"github.com/agentstation/starmap/pkg/errors"
)

// WithPermissionClock supplies one qualified time and uncertainty sample for each permission check.
// The callback reads cached evidence and supports concurrent calls. It starts no I/O.
// It governs admission, permission relay, and permission status independently of the scheduler clock.
// Use WithPermissionClockUncertainty only when this option is absent. The caller owns native clock qualification.
func WithPermissionClock(sample func() permission.ClockReading) Option {
	return func(o *options) error {
		if sample == nil {
			return &errors.ValidationError{Field: "permission_clock", Message: "is required"}
		}
		o.permissionClockReading = sample
		return nil
	}
}

// WithPermissionClockUncertainty supplies the host's cached clock assurance.
// The callback reads cached memory only. It reports uncertainty for WithClock's time
// and false when its evidence is absent or expired. No callback means unknown.
func WithPermissionClockUncertainty(sample func() (time.Duration, bool)) Option {
	return func(o *options) error {
		if sample == nil {
			return &errors.ValidationError{Field: "permission_clock", Message: "is required"}
		}
		o.permissionClockUncertainty = sample
		return nil
	}
}

func (r *Runtime) readPermissionClock() permission.ClockReading {
	if r.config.permissionClockReading != nil {
		return r.config.permissionClockReading()
	}
	if r.config.permissionClockUncertainty == nil {
		return permission.ClockReading{}
	}
	uncertainty, known := r.config.permissionClockUncertainty()
	return permission.ClockReading{Time: r.config.now(), Uncertainty: uncertainty, Known: known}
}
