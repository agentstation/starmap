// Package hostclock reads native clock evidence for explicit permission-clock refreshes.
// It starts no background work and changes no clock or time-service setting.
package hostclock

import (
	"context"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/permission"
	"github.com/agentstation/starmap/pkg/errors"
)

// Observe queries the host clock's current synchronization evidence.
// Call it through ClockCache.Refresh, outside permission checks and inference requests.
// The deployment must qualify its time service and counter error bounds separately.
// Unsupported platforms and unqualified clock states return no usable time.
func Observe(ctx context.Context) (permission.ClockReading, error) {
	if ctx == nil {
		return permission.ClockReading{}, invalidClock("an observation context is required")
	}
	if err := ctx.Err(); err != nil {
		return permission.ClockReading{}, err
	}
	return observeNative(ctx)
}

// Elapsed reads a nonnegative native elapsed counter that includes system sleep.
// It reads the counter without file or network I/O. Unsupported platforms return false.
// The deployment must qualify how the counter measures time and bounds rate error.
func Elapsed() (time.Duration, bool) { return elapsedNative() }

func invalidClock(message string) error {
	return &errors.ValidationError{Field: "permission_clock", Message: message}
}
