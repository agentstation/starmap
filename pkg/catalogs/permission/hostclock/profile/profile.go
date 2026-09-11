// Package profile defines host-selected bounds for native permission clocks.
// Validation starts no observation and does not qualify the declared bounds.
package profile

import (
	"context"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/permission"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// Disabled leaves permission time unqualified unless the host supplies another clock.
	Disabled = "disabled"
	// Native selects the current platform's native clock evidence.
	Native = "native"
	// MaxWindowsSourceAge limits the supported Windows synchronization history.
	MaxWindowsSourceAge = 24 * time.Hour
)

// Config contains declared host bounds without a native adapter dependency.
// An empty Source has the same meaning as Disabled. Native bounds require qualification.
type Config struct {
	// Source selects Disabled or Native. An empty value selects Disabled.
	Source string
	// RefreshInterval must be positive and less than half MaxAge.
	RefreshInterval time.Duration
	// MaxAge bounds cached evidence age to at most five minutes.
	MaxAge time.Duration
	// MaxDriftPPM bounds counter rate error below one million parts per million.
	MaxDriftPPM uint32
	// CounterUncertainty bounds each reading error with a positive duration.
	CounterUncertainty time.Duration
	// Windows holds the additional source bounds required on Windows.
	Windows Windows
}

// Windows contains declared error bounds for the W32Time synchronization source.
// Hosts can include these values in a shared profile. Windows requires all three.
type Windows struct {
	// MaxSourceAge bounds synchronization age to at most one day.
	MaxSourceAge time.Duration
	// MaxSourceDriftPPM bounds source rate error below one million parts per million.
	MaxSourceDriftPPM uint32
	// SourceUncertainty adds positive source error of at most thirty seconds.
	SourceUncertainty time.Duration
}

// Validate checks the profile without reading a clock or starting a worker.
// Native profiles need positive counter uncertainty and valid cache scheduling bounds.
// Absent Windows bounds remain valid here. The Windows host requires them at composition.
func (c Config) Validate() error {
	if c.Source == "" || c.Source == Disabled {
		return nil
	}
	if c.Source != Native {
		return invalid("source must be disabled or native")
	}
	if c.CounterUncertainty <= 0 {
		return invalid("native counter uncertainty must be positive")
	}
	// The cache owns its bounds. Its passive constructor validates them without calling these functions.
	_, err := permission.NewClockMonitor(permission.ClockCacheConfig{
		Observe: func(context.Context) (permission.ClockReading, error) { return permission.ClockReading{}, nil },
		Elapsed: func() (time.Duration, bool) { return 0, false },
		MaxAge:  c.MaxAge, MaxDriftPPM: c.MaxDriftPPM, CounterUncertainty: c.CounterUncertainty,
	}, c.RefreshInterval)
	if err != nil {
		return err
	}
	if c.Windows != (Windows{}) {
		return c.Windows.Validate()
	}
	return nil
}

// Validate checks the Windows source bounds without a platform dependency.
func (w Windows) Validate() error {
	if w.MaxSourceAge <= 0 || w.MaxSourceAge > MaxWindowsSourceAge {
		return invalid("Windows source age must be positive and within one day")
	}
	if w.MaxSourceDriftPPM == 0 || w.MaxSourceDriftPPM >= 1_000_000 {
		return invalid("Windows source drift must be positive and below one million parts per million")
	}
	if w.SourceUncertainty <= 0 || w.SourceUncertainty > catalogs.MaxCatalogPermissionClockUncertainty {
		return invalid("Windows source uncertainty must be positive and within thirty seconds")
	}
	return nil
}

func invalid(message string) error {
	return &errors.ValidationError{Field: "permission_clock.profile", Message: message}
}
