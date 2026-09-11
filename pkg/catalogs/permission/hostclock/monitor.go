package hostclock

import (
	"runtime"

	"github.com/agentstation/starmap/pkg/catalogs/permission"
	"github.com/agentstation/starmap/pkg/catalogs/permission/hostclock/profile"
)

// NewMonitor selects native functions and validates a declared host profile without I/O.
// Disabled profiles return nil. The host must start and close a returned monitor explicitly.
// Configuration does not qualify the time service or its error bounds.
func NewMonitor(config profile.Config) (*permission.ClockMonitor, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if config.Source == "" || config.Source == profile.Disabled {
		return nil, nil
	}
	observe := Observe
	switch runtime.GOOS {
	case "windows":
		observer, err := NewWindowsObserver(WindowsProfile(config.Windows))
		if err != nil {
			return nil, err
		}
		observe = observer.Observe
	case "linux", "darwin":
	default:
		return nil, invalidClock("native permission clocks are unsupported on this platform")
	}
	return permission.NewClockMonitor(permission.ClockCacheConfig{
		Observe: observe, Elapsed: Elapsed,
		MaxAge: config.MaxAge, MaxDriftPPM: config.MaxDriftPPM, CounterUncertainty: config.CounterUncertainty,
	}, config.RefreshInterval)
}
