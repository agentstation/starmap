package config

import (
	"strconv"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/permission/hostclock/profile"
	"github.com/agentstation/starmap/pkg/errors"
)

// Canonical clock settings belong to one host and require restart after changes.
const (
	// PermissionClockSource selects disabled or native permission clock evidence.
	PermissionClockSource = Prefix + "CATALOG_PERMISSION_CLOCK_SOURCE"
	// PermissionClockRefreshInterval sets the background observation period.
	PermissionClockRefreshInterval = Prefix + "CATALOG_PERMISSION_CLOCK_REFRESH_INTERVAL"
	// PermissionClockMaxAge bounds the lifetime of a cached observation.
	PermissionClockMaxAge = Prefix + "CATALOG_PERMISSION_CLOCK_MAX_AGE"
	// PermissionClockMaxDriftPPM bounds elapsed-counter rate error.
	PermissionClockMaxDriftPPM = Prefix + "CATALOG_PERMISSION_CLOCK_MAX_DRIFT_PPM"
	// PermissionClockCounterUncertainty bounds the error of each counter reading.
	PermissionClockCounterUncertainty = Prefix + "CATALOG_PERMISSION_CLOCK_COUNTER_UNCERTAINTY"
	// PermissionClockWindowsMaxSourceAge bounds Windows synchronization age.
	PermissionClockWindowsMaxSourceAge = Prefix + "CATALOG_PERMISSION_CLOCK_WINDOWS_MAX_SOURCE_AGE"
	// PermissionClockWindowsMaxSourceDriftPPM bounds Windows source rate error.
	PermissionClockWindowsMaxSourceDriftPPM = Prefix + "CATALOG_PERMISSION_CLOCK_WINDOWS_MAX_SOURCE_DRIFT_PPM"
	// PermissionClockWindowsSourceUncertainty bounds additional Windows source error.
	PermissionClockWindowsSourceUncertainty = Prefix + "CATALOG_PERMISSION_CLOCK_WINDOWS_SOURCE_UNCERTAINTY"
)

func clockSettings() []setting {
	return []setting{
		{name: PermissionClockSource, flag: "catalog-permission-clock-source", capture: captureClockSource},
		{name: PermissionClockRefreshInterval, flag: "catalog-permission-clock-refresh-interval", capture: captureDuration(PermissionClockRefreshInterval, func(c *Config, v time.Duration) { c.PermissionClock.RefreshInterval = v })},
		{name: PermissionClockMaxAge, flag: "catalog-permission-clock-max-age", capture: captureDuration(PermissionClockMaxAge, func(c *Config, v time.Duration) { c.PermissionClock.MaxAge = v })},
		{name: PermissionClockMaxDriftPPM, flag: "catalog-permission-clock-max-drift-ppm", capture: captureClockDrift(PermissionClockMaxDriftPPM, func(c *Config, v uint32) { c.PermissionClock.MaxDriftPPM = v })},
		{name: PermissionClockCounterUncertainty, flag: "catalog-permission-clock-counter-uncertainty", capture: captureDuration(PermissionClockCounterUncertainty, func(c *Config, v time.Duration) { c.PermissionClock.CounterUncertainty = v })},
		{name: PermissionClockWindowsMaxSourceAge, flag: "catalog-permission-clock-windows-max-source-age", capture: captureDuration(PermissionClockWindowsMaxSourceAge, func(c *Config, v time.Duration) { c.PermissionClock.Windows.MaxSourceAge = v })},
		{name: PermissionClockWindowsMaxSourceDriftPPM, flag: "catalog-permission-clock-windows-max-source-drift-ppm", capture: captureClockDrift(PermissionClockWindowsMaxSourceDriftPPM, func(c *Config, v uint32) { c.PermissionClock.Windows.MaxSourceDriftPPM = v })},
		{name: PermissionClockWindowsSourceUncertainty, flag: "catalog-permission-clock-windows-source-uncertainty", capture: captureDuration(PermissionClockWindowsSourceUncertainty, func(c *Config, v time.Duration) { c.PermissionClock.Windows.SourceUncertainty = v })},
	}
}

func captureClockSource(value string, c *Config) error {
	if value != profile.Disabled && value != profile.Native {
		return &errors.ValidationError{Field: PermissionClockSource, Message: "must be disabled or native"}
	}
	c.PermissionClock.Source = value
	return nil
}

func captureClockDrift(name string, assign func(*Config, uint32)) func(string, *Config) error {
	return func(value string, c *Config) error {
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil || parsed == 0 {
			return &errors.ValidationError{Field: name, Message: "must be a positive 32-bit integer"}
		}
		assign(c, uint32(parsed))
		return nil
	}
}

func describeClock(d *Descriptor) {
	d.Scope, d.Mutability, d.DefaultMeaning = NodeScope, "restart", "operator-qualified bound"
	d.Applicability = []string{"native permission clock"}
	switch d.Name {
	case PermissionClockSource:
		d.Description = "Selects native permission clock evidence. Native mode requires qualified bounds for this host."
		d.Default, d.DefaultMeaning = profile.Disabled, "no native observations"
		d.AllowedValues = []string{profile.Disabled, profile.Native}
		d.Applicability = nil
	case PermissionClockRefreshInterval:
		d.Description = "Sets the background observation period. It must be less than half the maximum cache age."
		d.Type, d.Unit = DurationValue, "duration"
	case PermissionClockMaxAge:
		d.Description = "Bounds cached observation age to at most five minutes."
		d.Type, d.Unit = DurationValue, "duration"
	case PermissionClockMaxDriftPPM:
		d.Description = "Bounds elapsed-counter rate error in parts per million, below one million."
		d.Type, d.Unit = IntegerValue, "parts per million"
	case PermissionClockCounterUncertainty:
		d.Description = "Bounds each elapsed-counter reading error. Supply a positive duration, at most thirty seconds."
		d.Type, d.Unit = DurationValue, "duration"
	case PermissionClockWindowsMaxSourceAge:
		d.Description = "Bounds Windows synchronization age to at most one day."
		d.Type, d.Unit = DurationValue, "duration"
		d.Applicability = []string{"native permission clock on Windows"}
	case PermissionClockWindowsMaxSourceDriftPPM:
		d.Description = "Bounds Windows synchronization source drift in parts per million, below one million."
		d.Type, d.Unit = IntegerValue, "parts per million"
		d.Applicability = []string{"native permission clock on Windows"}
	case PermissionClockWindowsSourceUncertainty:
		d.Description = "Adds qualified Windows source error. Supply a positive duration, at most thirty seconds."
		d.Type, d.Unit = DurationValue, "duration"
		d.Applicability = []string{"native permission clock on Windows"}
	}
}
