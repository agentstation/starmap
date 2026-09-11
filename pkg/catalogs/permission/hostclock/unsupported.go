//go:build !linux && !darwin && !windows

package hostclock

import (
	"context"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/permission"
)

func observeNative(context.Context) (permission.ClockReading, error) {
	return permission.ClockReading{}, invalidClock("native clock observation is not supported on this platform")
}

func elapsedNative() (time.Duration, bool) { return 0, false }
