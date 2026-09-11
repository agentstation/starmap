//go:build windows

package hostclock

import (
	"context"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/permission"
)

// The supported Go runtimes use Windows interrupt time for their monotonic clock.
// It includes system sleep. Keep the monotonic component in this process-local epoch.
var windowsCounterStart = time.Now()

func elapsedNative() (time.Duration, bool) {
	elapsed := time.Since(windowsCounterStart)
	return elapsed, elapsed >= 0
}

func observeNative(context.Context) (permission.ClockReading, error) {
	return permission.ClockReading{}, invalidClock("Windows observation requires an explicit profile through NewWindowsObserver")
}
