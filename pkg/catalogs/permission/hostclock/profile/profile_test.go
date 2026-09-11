package profile_test

import (
	"errors"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/permission/hostclock/profile"
	starmaperrors "github.com/agentstation/starmap/pkg/errors"
)

func qualifiedProfile() profile.Config {
	return profile.Config{Source: profile.Native, MaxAge: time.Minute, RefreshInterval: 10 * time.Second, MaxDriftPPM: 500, CounterUncertainty: time.Microsecond}
}

func TestNativeProfileRejectsIncompleteOrUnsafeBounds(t *testing.T) {
	cases := []struct {
		name   string
		change func(*profile.Config)
	}{
		{"unknown source", func(c *profile.Config) { c.Source = "unknown" }},
		{"no interval", func(c *profile.Config) { c.RefreshInterval = 0 }},
		{"half age interval", func(c *profile.Config) { c.RefreshInterval = c.MaxAge / 2 }},
		{"no age", func(c *profile.Config) { c.MaxAge = 0 }},
		{"excess age", func(c *profile.Config) { c.MaxAge = 5*time.Minute + 1 }},
		{"no drift", func(c *profile.Config) { c.MaxDriftPPM = 0 }},
		{"excess drift", func(c *profile.Config) { c.MaxDriftPPM = 1_000_000 }},
		{"no counter uncertainty", func(c *profile.Config) { c.CounterUncertainty = 0 }},
		{"negative counter uncertainty", func(c *profile.Config) { c.CounterUncertainty = -1 }},
		{"excess counter uncertainty", func(c *profile.Config) { c.CounterUncertainty = 30*time.Second + 1 }},
		{"incomplete Windows bounds", func(c *profile.Config) { c.Windows.MaxSourceAge = time.Hour }},
		{"excess Windows age", func(c *profile.Config) {
			c.Windows = profile.Windows{MaxSourceAge: 24*time.Hour + 1, MaxSourceDriftPPM: 500, SourceUncertainty: time.Millisecond}
		}},
		{"excess Windows drift", func(c *profile.Config) {
			c.Windows = profile.Windows{MaxSourceAge: time.Hour, MaxSourceDriftPPM: 1_000_000, SourceUncertainty: time.Millisecond}
		}},
		{"excess Windows uncertainty", func(c *profile.Config) {
			c.Windows = profile.Windows{MaxSourceAge: time.Hour, MaxSourceDriftPPM: 500, SourceUncertainty: 30*time.Second + 1}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qualifiedProfile()
			tc.change(&c)
			var invalid *starmaperrors.ValidationError
			if err := c.Validate(); !errors.As(err, &invalid) {
				t.Fatalf("validation = %v, want typed rejection", err)
			}
		})
	}
}

func TestProfileAcceptsExplicitPortableBoundsAndDisabledState(t *testing.T) {
	portable := qualifiedProfile()
	if err := portable.Validate(); err != nil {
		t.Fatal(err)
	}
	portable.Windows = profile.Windows{MaxSourceAge: 24 * time.Hour, MaxSourceDriftPPM: 999_999, SourceUncertainty: 30 * time.Second}
	if err := portable.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []profile.Config{{}, {Source: profile.Disabled}, {Source: profile.Disabled, MaxAge: -1}} {
		if err := c.Validate(); err != nil {
			t.Fatalf("disabled profile: %v", err)
		}
	}
}
