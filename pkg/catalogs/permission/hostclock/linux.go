//go:build linux

package hostclock

import (
	"context"
	"math"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/permission"
	"github.com/agentstation/starmap/pkg/errors"
	"golang.org/x/sys/unix"
)

func observeNative(ctx context.Context) (permission.ClockReading, error) {
	return observeLinux(ctx, unix.Adjtimex)
}

func observeLinux(ctx context.Context, query func(*unix.Timex) (int, error)) (permission.ClockReading, error) {
	var sample unix.Timex
	// Zero modes requests a read without selecting clock discipline or correction.
	state, err := query(&sample)
	if err != nil {
		return permission.ClockReading{}, errors.WrapResource("observe", "host clock", "", err)
	}
	if err := ctx.Err(); err != nil {
		return permission.ClockReading{}, err
	}
	return linuxReading(state, sample)
}

func linuxReading(state int, sample unix.Timex) (permission.ClockReading, error) {
	const unsafeStatus = unix.STA_UNSYNC | unix.STA_CLOCKERR | unix.STA_INS | unix.STA_DEL
	if state != unix.TIME_OK || sample.Status&unsafeStatus != 0 {
		return permission.ClockReading{}, invalidClock("host clock synchronization or leap state is unqualified")
	}
	unit := time.Microsecond
	if sample.Status&unix.STA_NANO != 0 {
		unit = time.Nanosecond
	}
	if sample.Time.Usec < 0 || sample.Time.Usec >= int64(time.Second/unit) {
		return permission.ClockReading{}, invalidClock("host clock timestamp fraction is invalid")
	}
	stamp := time.Unix(sample.Time.Sec, sample.Time.Usec*int64(unit)).UTC()
	if stamp.Year() < 1 || stamp.Year() > 9999 {
		return permission.ClockReading{}, invalidClock("host clock timestamp is outside the supported UTC range")
	}
	limit := catalogs.MaxCatalogPermissionClockUncertainty
	if sample.Maxerror < 0 || sample.Esterror < 0 || sample.Precision < 0 ||
		sample.Maxerror > int64(limit/time.Microsecond) || sample.Esterror > int64(limit/time.Microsecond) ||
		sample.Precision > int64(limit/time.Microsecond) ||
		sample.Offset < -int64(limit/unit) || sample.Offset > int64(limit/unit) {
		return permission.ClockReading{}, invalidClock("host clock error evidence is outside the permission bound")
	}
	offset := sample.Offset
	if offset < 0 {
		offset = -offset
	}
	// Kernel error and precision fields use microseconds even in nanosecond mode.
	// Include the pending phase correction and timestamp quantization in the bound.
	bound := time.Duration(max(sample.Maxerror, sample.Esterror)+sample.Precision)*time.Microsecond + time.Duration(offset)*unit + unit
	if bound > limit {
		return permission.ClockReading{}, invalidClock("combined host clock uncertainty exceeds the permission bound")
	}
	return permission.ClockReading{Time: stamp, Uncertainty: bound, Known: true}, nil
}

func elapsedNative() (time.Duration, bool) {
	var value unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_BOOTTIME, &value); err != nil {
		return 0, false
	}
	return linuxElapsed(value)
}

func linuxElapsed(value unix.Timespec) (time.Duration, bool) {
	if value.Sec < 0 || value.Nsec < 0 || value.Nsec >= int64(time.Second) ||
		value.Sec > (math.MaxInt64-value.Nsec)/int64(time.Second) {
		return 0, false
	}
	return time.Duration(value.Sec)*time.Second + time.Duration(value.Nsec), true
}
