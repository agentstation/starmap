//go:build darwin

package hostclock

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/permission"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

// darwinNTPTime matches the 64-bit ntptimeval layout in the macOS SDK.
type darwinNTPTime struct {
	Seconds, Nanoseconds, MaxError, EstError, TAI int64
	State                                         int32
	Padding                                       int32
}

// The process retains this system-library reference while the function remains live.
var nativeDarwinObservation = sync.OnceValues(func() (func(*darwinNTPTime) int32, error) {
	library, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	symbol, err := purego.Dlsym(library, "ntp_gettime")
	if err != nil {
		_ = purego.Dlclose(library)
		return nil, err
	}
	var query func(*darwinNTPTime) int32
	purego.RegisterFunc(&query, symbol)
	return query, nil
})

func observeNative(ctx context.Context) (permission.ClockReading, error) {
	query, err := nativeDarwinObservation()
	if err != nil {
		return permission.ClockReading{}, errors.WrapResource("load", "host clock API", "", err)
	}
	if err := ctx.Err(); err != nil {
		return permission.ClockReading{}, err
	}
	var sample darwinNTPTime
	if query(&sample) != 0 {
		return permission.ClockReading{}, invalidClock("host clock observation failed")
	}
	if err := ctx.Err(); err != nil {
		return permission.ClockReading{}, err
	}
	return darwinReading(sample)
}

func darwinReading(sample darwinNTPTime) (permission.ClockReading, error) {
	const synchronized = 0
	const timestampUncertainty = time.Microsecond
	if sample.State != synchronized {
		return permission.ClockReading{}, invalidClock("host clock synchronization or leap state is unqualified")
	}
	if sample.Nanoseconds < 0 || sample.Nanoseconds >= int64(time.Second) {
		return permission.ClockReading{}, invalidClock("host clock timestamp fraction is invalid")
	}
	stamp := time.Unix(sample.Seconds, sample.Nanoseconds).UTC()
	if stamp.Year() < 1 || stamp.Year() > 9999 {
		return permission.ClockReading{}, invalidClock("host clock timestamp is outside the supported UTC range")
	}
	limit := int64((catalogs.MaxCatalogPermissionClockUncertainty - timestampUncertainty) / time.Microsecond)
	if sample.MaxError < 0 || sample.EstError < 0 || sample.MaxError > limit || sample.EstError > limit {
		return permission.ClockReading{}, invalidClock("host clock error evidence is outside the permission bound")
	}
	bound := time.Duration(max(sample.MaxError, sample.EstError))*time.Microsecond + timestampUncertainty
	return permission.ClockReading{Time: stamp, Uncertainty: bound, Known: true}, nil
}

func elapsedNative() (time.Duration, bool) {
	var value unix.Timespec
	// macOS maps this clock to mach_continuous_time, which includes system sleep.
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC_RAW, &value); err != nil {
		return 0, false
	}
	if value.Sec < 0 || value.Nsec < 0 || value.Nsec >= int64(time.Second) ||
		value.Sec > (math.MaxInt64-value.Nsec)/int64(time.Second) {
		return 0, false
	}
	return time.Duration(value.Sec)*time.Second + time.Duration(value.Nsec), true
}
