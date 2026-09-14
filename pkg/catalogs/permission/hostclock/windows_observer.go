package hostclock

import (
	"context"
	"math"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/permission"
	clockprofile "github.com/agentstation/starmap/pkg/catalogs/permission/hostclock/profile"
)

const (
	windowsTick             = 100 * time.Nanosecond
	maxWindowsSourceAge     = clockprofile.MaxWindowsSourceAge
	windowsUnixEpochSeconds = 11_644_473_600
	clockPartsPerMillion    = 1_000_000
)

// WindowsProfile supplies deployment-qualified bounds for the W32Time source.
// It does not qualify the source or change Windows time-service settings.
type WindowsProfile struct {
	// MaxSourceAge bounds the actual age of the last successful synchronization.
	// It must be positive and cannot exceed one day.
	MaxSourceAge time.Duration
	// MaxSourceDriftPPM bounds source-clock rate error between synchronizations.
	// It must be positive and below one million parts per million.
	MaxSourceDriftPPM uint32
	// SourceUncertainty bounds UTC error beyond the service's reported evidence.
	// It must be positive and within the thirty-second permission uncertainty limit.
	SourceUncertainty time.Duration
}

// WindowsObserver queries the local Windows time service with explicit error bounds.
// NewWindowsObserver starts no I/O. Use Observe for scheduled refreshes outside requests.
type WindowsObserver struct{ profile WindowsProfile }

// NewWindowsObserver validates a source profile without contacting a time service.
// Other platforms can validate profiles but cannot query Windows.
func NewWindowsObserver(profile WindowsProfile) (*WindowsObserver, error) {
	if err := clockprofile.Windows(profile).Validate(); err != nil {
		return nil, err
	}
	return &WindowsObserver{profile: profile}, nil
}

// Observe reads one local W32Time status and derives UTC with its complete error bound.
// It changes no clock settings and stops when the observation context ends.
func (o *WindowsObserver) Observe(ctx context.Context) (permission.ClockReading, error) {
	if o == nil || o.profile.MaxSourceAge <= 0 || ctx == nil {
		return permission.ClockReading{}, invalidClock("a Windows observer and context are required")
	}
	if err := ctx.Err(); err != nil {
		return permission.ClockReading{}, err
	}
	return observeWindowsEvidence(ctx, o.profile, readWindowsEvidence)
}

// observeWindowsEvidence keeps validation with the observer and delegates native evidence acquisition.
func observeWindowsEvidence(ctx context.Context, profile WindowsProfile, read func(context.Context) (windowsEvidence, error)) (permission.ClockReading, error) {
	sample, err := read(ctx)
	if err != nil {
		return permission.ClockReading{}, err
	}
	if err := ctx.Err(); err != nil {
		return permission.ClockReading{}, err
	}
	return windowsReading(profile, sample)
}

type windowsEvidence struct {
	LeapIndicator    uint32
	Stratum          uint32
	LastSyncTicks    uint64
	TimeLastGoodSync uint64
	RootDispersion   uint64
	RootDelay        int64
	PhaseOffset      int64
	ClockPrecision   int32
	Source           string
	State            uint32
	Flags            uint32
	LastSyncResult   uint32
}

func windowsReading(profile WindowsProfile, sample windowsEvidence) (permission.ClockReading, error) {
	if sample.LeapIndicator != 0 || sample.Stratum == 0 || sample.Stratum >= 16 ||
		sample.State != 2 || sample.LastSyncResult != 0 || sample.Flags & ^uint32(6) != 0 || sample.Source == "" {
		return permission.ClockReading{}, invalidClock("Windows time-service synchronization evidence is unqualified")
	}
	if sample.LastSyncTicks == 0 || sample.TimeLastGoodSync > uint64(maxWindowsSourceAge/windowsTick) ||
		sample.LastSyncTicks > math.MaxUint64-sample.TimeLastGoodSync {
		return permission.ClockReading{}, invalidClock("Windows synchronization timestamp or age is invalid")
	}
	// One status binds the absolute last-sync timestamp to elapsed time since that sync.
	// Both fields use 100-nanosecond units. Only LastSyncTicks has a calendar epoch.
	ticks := sample.LastSyncTicks + sample.TimeLastGoodSync
	const ticksPerSecond = uint64(time.Second / windowsTick)
	seconds := ticks / ticksPerSecond
	// The final second of year 9999 bounds the conversion before time construction.
	const maximumWindowsSeconds = 265_046_774_399
	if seconds > maximumWindowsSeconds {
		return permission.ClockReading{}, invalidClock("Windows synchronization timestamp is outside the supported UTC range")
	}
	stamp := time.Unix(int64(seconds)-windowsUnixEpochSeconds, int64(ticks%ticksPerSecond)*int64(windowsTick)).UTC()
	if stamp.Year() < 1 || stamp.Year() > 9999 {
		return permission.ClockReading{}, invalidClock("Windows synchronization timestamp is outside the supported UTC range")
	}
	const limit = catalogs.MaxCatalogPermissionClockUncertainty
	const limitTicks = int64(limit / windowsTick)
	if sample.RootDispersion > uint64(limitTicks) || sample.RootDelay < -limitTicks || sample.RootDelay > limitTicks ||
		sample.PhaseOffset < -limitTicks || sample.PhaseOffset > limitTicks || sample.ClockPrecision >= 5 {
		return permission.ClockReading{}, invalidClock("Windows time-service error evidence exceeds the permission bound")
	}
	age := time.Duration(sample.TimeLastGoodSync) * windowsTick
	// The returned UTC combines two timestamp fields. Bound both readings' precision.
	precisionError := 2 * (windowsPrecision(sample.ClockPrecision) + windowsTick)
	drift, valid := windowsSourceDrift(age+precisionError, profile.MaxSourceDriftPPM, limit)
	if !valid || age+precisionError+drift >= profile.MaxSourceAge {
		return permission.ClockReading{}, invalidClock("Windows source age or drift exceeds the qualified profile")
	}
	delay, phase := sample.RootDelay, sample.PhaseOffset
	if delay < 0 {
		delay = -delay
	}
	if phase < 0 {
		phase = -phase
	}
	bound := profile.SourceUncertainty + time.Duration(sample.RootDispersion)*windowsTick +
		time.Duration((delay+1)/2)*windowsTick + time.Duration(phase)*windowsTick + precisionError + drift
	if bound > limit {
		return permission.ClockReading{}, invalidClock("combined Windows clock uncertainty exceeds the permission bound")
	}
	return permission.ClockReading{Time: stamp, Uncertainty: bound, Known: true}, nil
}

func windowsSourceDrift(age time.Duration, ppm uint32, limit time.Duration) (time.Duration, bool) {
	divisor := int64(clockPartsPerMillion - ppm)
	whole, remainder := int64(age)/divisor, int64(age)%divisor
	if whole > int64(limit)/int64(ppm) {
		return 0, false
	}
	drift := time.Duration(whole*int64(ppm) + (remainder*int64(ppm)+divisor-1)/divisor)
	return drift, drift <= limit
}

func windowsPrecision(exponent int32) time.Duration {
	if exponent >= 0 {
		return time.Second << uint(exponent)
	}
	if exponent <= -30 {
		return time.Nanosecond
	}
	divisor := int64(1) << uint(-exponent)
	return time.Duration((int64(time.Second) + divisor - 1) / divisor)
}
