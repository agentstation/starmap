package hostclock

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

func TestWindowsObserverRequiresProfile(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*WindowsProfile)
	}{
		{"missing age", func(p *WindowsProfile) { p.MaxSourceAge = 0 }},
		{"excessive age", func(p *WindowsProfile) { p.MaxSourceAge = 25 * time.Hour }},
		{"missing drift", func(p *WindowsProfile) { p.MaxSourceDriftPPM = 0 }},
		{"excessive drift", func(p *WindowsProfile) { p.MaxSourceDriftPPM = 1_000_000 }},
		{"missing uncertainty", func(p *WindowsProfile) { p.SourceUncertainty = 0 }},
		{"excessive uncertainty", func(p *WindowsProfile) { p.SourceUncertainty = 31 * time.Second }},
	} {
		t.Run(test.name, func(t *testing.T) {
			profile := windowsTestProfile()
			test.change(&profile)
			if observer, err := NewWindowsObserver(profile); observer != nil || err == nil {
				t.Fatalf("invalid profile accepted: %v", err)
			}
		})
	}
	observer, err := NewWindowsObserver(windowsTestProfile())
	if err != nil {
		t.Fatal(err)
	}
	if reading, err := new(WindowsObserver).Observe(t.Context()); err == nil || reading.Known {
		t.Fatal("unconstructed observer accepted")
	}
	if reading, err := observer.Observe(nil); err == nil || reading.Known {
		t.Fatal("nil context accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if reading, err := observer.Observe(ctx); !errors.Is(err, context.Canceled) || reading.Known {
		t.Fatal("canceled context accepted")
	}
}

func windowsTestProfile() WindowsProfile {
	return WindowsProfile{MaxSourceAge: time.Hour, MaxSourceDriftPPM: 500, SourceUncertainty: 100 * time.Millisecond}
}

func windowsTestEvidence() windowsEvidence {
	return windowsEvidence{LeapIndicator: 0, Stratum: 2, LastSyncTicks: 134000000000000000, TimeLastGoodSync: 10_000_000, RootDispersion: 200_000, RootDelay: 100_000, PhaseOffset: -10_000, ClockPrecision: -10, Source: "qualified-time-source", State: 2}
}

func TestWindowsEvidenceBindsTimeAndUncertainty(t *testing.T) {
	p := windowsTestProfile()
	s := windowsTestEvidence()
	r, err := windowsReading(p, s)
	if err != nil {
		t.Fatal(err)
	}
	wantTime := time.Unix(1_755_526_400, 0).Add(time.Second).UTC()
	// Include source uncertainty, dispersion, half-delay, phase, precision, drift, and quantization.
	wantBound := 100*time.Millisecond + 20*time.Millisecond + 5*time.Millisecond + time.Millisecond + 2*976563*time.Nanosecond + 501228*time.Nanosecond + 200*time.Nanosecond
	if !r.Known || !r.Time.Equal(wantTime) || r.Uncertainty != wantBound {
		t.Fatalf("reading = %+v; want %s ± %s", r, wantTime, wantBound)
	}
	s.RootDelay = -s.RootDelay
	negative, err := windowsReading(p, s)
	if err != nil || negative != r {
		t.Fatalf("signed delay changed conservative bound: %+v, %v", negative, err)
	}
}

func TestWindowsEvidenceRefusesUnsafeSamples(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*windowsEvidence)
	}{
		{"unqualified leap", func(s *windowsEvidence) { s.LeapIndicator = 3 }},
		{"pending leap", func(s *windowsEvidence) { s.LeapIndicator = 1 }},
		{"stratum zero", func(s *windowsEvidence) { s.Stratum = 0 }},
		{"stratum unsynchronized", func(s *windowsEvidence) { s.Stratum = 16 }},
		{"hold", func(s *windowsEvidence) { s.State = 1 }},
		{"spike", func(s *windowsEvidence) { s.State = 3 }},
		{"failed last sync", func(s *windowsEvidence) { s.LastSyncResult = 2 }},
		{"hardware flag", func(s *windowsEvidence) { s.Flags = 1 }},
		{"unknown flag", func(s *windowsEvidence) { s.Flags = 8 }},
		{"missing source", func(s *windowsEvidence) { s.Source = "" }},
		{"missing sync time", func(s *windowsEvidence) { s.LastSyncTicks = 0 }},
		{"timestamp overflow", func(s *windowsEvidence) { s.LastSyncTicks = math.MaxUint64 }},
		{"unsupported calendar", func(s *windowsEvidence) { s.LastSyncTicks = math.MaxUint64 - 10_000_000 }},
		{"excessive age", func(s *windowsEvidence) { s.TimeLastGoodSync = uint64(time.Hour / (100 * time.Nanosecond)) }},
		{"age overflow", func(s *windowsEvidence) { s.TimeLastGoodSync = math.MaxUint64 }},
		{"delay overflow", func(s *windowsEvidence) { s.RootDelay = math.MinInt64 }},
		{"phase overflow", func(s *windowsEvidence) { s.PhaseOffset = math.MaxInt64 }},
		{"dispersion overflow", func(s *windowsEvidence) { s.RootDispersion = math.MaxUint64 }},
		{"precision overflow", func(s *windowsEvidence) { s.ClockPrecision = math.MaxInt32 }},
		{"combined uncertainty", func(s *windowsEvidence) { s.RootDispersion = 300_000_000 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := windowsTestEvidence()
			test.change(&s)
			if r, err := windowsReading(windowsTestProfile(), s); err == nil || r.Known {
				t.Fatalf("unsafe sample accepted: %+v, %v", r, err)
			}
		})
	}
}

func TestWindowsSourceAgeAndDriftBounds(t *testing.T) {
	p := windowsTestProfile()
	p.MaxSourceAge = 24 * time.Hour
	p.MaxSourceDriftPPM = 999_999
	s := windowsTestEvidence()
	s.TimeLastGoodSync = uint64(23 * time.Hour / (100 * time.Nanosecond))
	if r, err := windowsReading(p, s); err == nil || r.Known {
		t.Fatal("overflowing drift accepted")
	}
	p = windowsTestProfile()
	p.MaxSourceAge = time.Second
	s = windowsTestEvidence()
	s.TimeLastGoodSync = 9_998_000
	if r, err := windowsReading(p, s); err == nil || r.Known {
		t.Fatal("actual source age beyond the profile accepted")
	}
	p.MaxSourceAge = 500 * time.Microsecond
	s.TimeLastGoodSync = 0
	if r, err := windowsReading(p, s); err == nil || r.Known {
		t.Fatal("timestamp precision beyond source age bound accepted")
	}
}
