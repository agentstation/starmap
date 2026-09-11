package hostclock_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/permission/hostclock"
	"github.com/agentstation/starmap/pkg/catalogs/permission/hostclock/profile"
)

func TestMonitorFactoryIsPassiveAndDisabledByDefault(t *testing.T) {
	for _, c := range []profile.Config{{}, {Source: profile.Disabled}} {
		if m, err := hostclock.NewMonitor(c); err != nil || m != nil {
			t.Fatalf("disabled monitor = %v, %v", m, err)
		}
	}
	if _, err := hostclock.NewMonitor(profile.Config{Source: profile.Native}); err == nil {
		t.Fatal("accepted incomplete native profile")
	}
	c := profile.Config{Source: profile.Native, MaxAge: time.Minute, RefreshInterval: 10 * time.Second, MaxDriftPPM: 500, CounterUncertainty: time.Microsecond,
		Windows: profile.Windows{MaxSourceAge: time.Hour, MaxSourceDriftPPM: 500, SourceUncertainty: time.Millisecond}}
	m, err := hostclock.NewMonitor(c)
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
		if err != nil {
			t.Fatal(err)
		}
		defer m.Close()
		if status := m.Status(); status.Running || status.Known || status.Attempts != 0 || m.Read().Known {
			t.Fatal("factory started clock work or supplied evidence")
		}
	default:
		if err == nil || m != nil {
			t.Fatal("unsupported platform accepted native profile")
		}
	}
}

func TestWindowsMonitorFactoryRequiresSourceBounds(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows composition contract")
	}
	c := profile.Config{Source: profile.Native, MaxAge: time.Minute, RefreshInterval: 10 * time.Second, MaxDriftPPM: 500, CounterUncertainty: time.Microsecond}
	if _, err := hostclock.NewMonitor(c); err == nil {
		t.Fatal("Windows accepted absent synchronization source bounds")
	}
}
