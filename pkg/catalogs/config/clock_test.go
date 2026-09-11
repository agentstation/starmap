package config_test

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/catalogs/permission/hostclock/profile"
)

func TestPermissionClockSettingsPreserveHostProfile(t *testing.T) {
	values := map[string]string{
		config.PermissionClockSource:                   "native",
		config.PermissionClockRefreshInterval:          "10s",
		config.PermissionClockMaxAge:                   "1m",
		config.PermissionClockMaxDriftPPM:              "100",
		config.PermissionClockCounterUncertainty:       "1ms",
		config.PermissionClockWindowsMaxSourceAge:      "1h",
		config.PermissionClockWindowsMaxSourceDriftPPM: "200",
		config.PermissionClockWindowsSourceUncertainty: "2ms",
	}
	parsed, err := config.Parse(values)
	if err != nil {
		t.Fatal(err)
	}
	want := profile.Config{Source: profile.Native, RefreshInterval: 10 * time.Second, MaxAge: time.Minute, MaxDriftPPM: 100, CounterUncertainty: time.Millisecond,
		Windows: profile.Windows{MaxSourceAge: time.Hour, MaxSourceDriftPPM: 200, SourceUncertainty: 2 * time.Millisecond}}
	if parsed.PermissionClock != want {
		t.Fatalf("profile = %+v, want %+v", parsed.PermissionClock, want)
	}
	if len(parsed.Options()) != 0 {
		t.Fatal("host profile became a runtime option before native composition")
	}
	if len(parsed.Configured()) != len(values) {
		t.Fatal("host settings lost presence")
	}
	for _, d := range config.Descriptors() {
		if want, found := values[d.Name]; found {
			if got, present := parsed.Value(d.Name); !present || got != want {
				t.Fatalf("lost %s", d.Name)
			}
			if d.Scope != config.NodeScope || d.Mutability != "restart" || d.SourceBinding != "" || d.Sensitive {
				t.Fatalf("unsafe clock descriptor: %+v", d)
			}
		}
	}
}

func TestPermissionClockSettingsRejectInvalidValues(t *testing.T) {
	for name, values := range map[string][]string{
		config.PermissionClockSource:                   {"", "system", "NATIVE"},
		config.PermissionClockRefreshInterval:          {"0s", "-1s", "bad"},
		config.PermissionClockMaxAge:                   {"0s", "-1s"},
		config.PermissionClockMaxDriftPPM:              {"0", "-1", "4294967296"},
		config.PermissionClockCounterUncertainty:       {"0s", "-1s"},
		config.PermissionClockWindowsMaxSourceAge:      {"0s", "-1s"},
		config.PermissionClockWindowsMaxSourceDriftPPM: {"0", "-1", "4294967296"},
		config.PermissionClockWindowsSourceUncertainty: {"0s", "-1s"},
	} {
		for _, value := range values {
			if _, err := config.Parse(map[string]string{name: value}); err == nil {
				t.Errorf("accepted %s=%s", name, value)
			}
		}
	}
}

func TestSourceReplacementPreservesIndependentClockSettings(t *testing.T) {
	resolved, err := config.Resolve(
		config.Layer{Name: "flags", Values: map[string]string{config.Source: "embedded", config.PermissionClockSource: "disabled"}},
		config.Layer{Name: "file", Values: map[string]string{config.Source: "starmap", config.SourceURL: "https://internal.example", config.PermissionClockSource: "native", config.PermissionClockMaxAge: "1m"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Config.PermissionClock.Source != profile.Disabled || resolved.Config.PermissionClock.MaxAge != time.Minute {
		t.Fatal("clock profile lost independent precedence")
	}
	if resolved.Origins[config.PermissionClockSource] != "flags" || resolved.Origins[config.PermissionClockMaxAge] != "file" {
		t.Fatal("clock setting origins were lost")
	}
	if _, present := resolved.Config.Value(config.SourceURL); present {
		t.Fatal("replaced catalog source retained transport settings")
	}
}
