package settings_test

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/catalog/settings"
	"github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/catalogs/permission"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/runtime"
)

func TestCompositionStartsAndClosesConfiguredNativeClock(t *testing.T) {
	parsed, err := config.Parse(map[string]string{
		config.Source: "embedded", config.AcquisitionEnabled: "false",
		config.PermissionClockSource: "native", config.PermissionClockRefreshInterval: "10s",
		config.PermissionClockMaxAge: "1m", config.PermissionClockMaxDriftPPM: "500",
		config.PermissionClockCounterUncertainty: "1us", config.PermissionClockWindowsMaxSourceAge: "1h",
		config.PermissionClockWindowsMaxSourceDriftPPM: "500", config.PermissionClockWindowsSourceUncertainty: "1ms",
	})
	if err != nil {
		t.Fatal(err)
	}
	composition := settings.Composition{Config: parsed, Base: clockRuntimeBase(t)}
	options, err := composition.Options()
	if err != nil {
		t.Fatal(err)
	}
	connected, err := runtime.Open(t.Context(), options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := connected.Close(); err != nil {
			t.Error(err)
		}
	})
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for connected.PermissionClockStatus().Attempts == 0 {
		select {
		case <-deadline.C:
			t.Fatal("configured clock made no observation")
		case <-ticker.C:
		}
	}
	// Native hosts can reject unqualified time. Startup still preserves catalog diagnostics.
	if !connected.PermissionClockStatus().Running || !connected.Status().CatalogAvailable {
		t.Fatal("clock lifecycle or catalog diagnostics unavailable")
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	if status := connected.PermissionClockStatus(); status.Running || status.Known {
		t.Fatal("closed runtime retained native time")
	}
}

func TestCompositionRejectsIncompleteClockBeforeRuntimeIO(t *testing.T) {
	parsed, err := config.Parse(map[string]string{config.PermissionClockSource: "native"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (settings.Composition{Config: parsed}).Options(); err == nil {
		t.Fatal("composition accepted missing host bounds")
	}
}

func TestCompositionDisabledClockOverridesBaseMonitor(t *testing.T) {
	var observations atomic.Uint64
	m, err := permission.NewClockMonitor(permission.ClockCacheConfig{
		Observe: func(context.Context) (permission.ClockReading, error) {
			observations.Add(1)
			return permission.ClockReading{}, nil
		},
		Elapsed: func() (time.Duration, bool) { return 0, true },
		MaxAge:  time.Minute, MaxDriftPPM: 500,
	}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	parsed, err := config.Parse(map[string]string{config.Source: "embedded", config.AcquisitionEnabled: "false", config.PermissionClockSource: "disabled"})
	if err != nil {
		t.Fatal(err)
	}
	base := append(clockRuntimeBase(t), runtime.WithPermissionClockMonitor(m))
	connected, err := (settings.Composition{Config: parsed, Base: base}).Open(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	if m.Status().Running || m.Status().Attempts != 0 || observations.Load() != 0 {
		t.Fatal("disabled base monitor started")
	}
	if err := m.Start(t.Context()); err != nil {
		t.Fatalf("composition took ownership of disabled base monitor: %v", err)
	}
}

func clockRuntimeBase(t *testing.T) []runtime.Option {
	t.Helper()
	return []runtime.Option{
		runtime.WithStateDirectory(filepath.Join(t.TempDir(), "state")),
		runtime.WithClientOptions(starmap.WithCatalogStore(storage.NewMemory()), starmap.WithCatalogPath(filepath.Join(t.TempDir(), "catalog"))),
	}
}
