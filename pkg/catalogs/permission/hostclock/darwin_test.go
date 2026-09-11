//go:build darwin

package hostclock

import (
	"math"
	"testing"
	"time"
	"unsafe"
)

func TestDarwinClockEvidence(t *testing.T) {
	if unsafe.Sizeof(darwinNTPTime{}) != 48 {
		t.Fatal("ntptimeval ABI size changed")
	}
	for _, test := range []struct {
		name   string
		change func(*darwinNTPTime)
	}{
		{name: "unsynchronized", change: func(v *darwinNTPTime) { v.State = 5 }},
		{name: "leap", change: func(v *darwinNTPTime) { v.State = 1 }},
		{name: "unknown state", change: func(v *darwinNTPTime) { v.State = 99 }},
		{name: "negative maximum", change: func(v *darwinNTPTime) { v.MaxError = -1 }},
		{name: "negative estimate", change: func(v *darwinNTPTime) { v.EstError = -1 }},
		{name: "overflow", change: func(v *darwinNTPTime) { v.MaxError = math.MaxInt64 }},
		{name: "excessive bound", change: func(v *darwinNTPTime) { v.EstError = 30_000_000 }},
		{name: "fraction negative", change: func(v *darwinNTPTime) { v.Nanoseconds = -1 }},
		{name: "fraction overflow", change: func(v *darwinNTPTime) { v.Nanoseconds = 1_000_000_000 }},
		{name: "invalid year", change: func(v *darwinNTPTime) { v.Seconds = math.MaxInt64 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			sample := darwinNTPTime{Seconds: 1_780_000_000, Nanoseconds: 12, MaxError: 100, EstError: 200}
			test.change(&sample)
			reading, err := darwinReading(sample)
			if err == nil || reading.Known {
				t.Fatalf("unsafe observation = %+v, %v", reading, err)
			}
		})
	}
	sample := darwinNTPTime{Seconds: 1_780_000_000, Nanoseconds: 12, MaxError: 100, EstError: 200}
	reading, err := darwinReading(sample)
	if err != nil || !reading.Known || !reading.Time.Equal(time.Unix(sample.Seconds, sample.Nanoseconds)) || reading.Uncertainty != 201*time.Microsecond {
		t.Fatalf("valid observation = %+v, %v", reading, err)
	}
}

func TestDarwinNativeObservation(t *testing.T) {
	wallBefore := time.Now().UTC()
	before, known := Elapsed()
	if !known {
		t.Fatal("sleep-inclusive counter unavailable")
	}
	reading, err := Observe(t.Context())
	wallAfter := time.Now().UTC()
	after, known := Elapsed()
	if !known || after < before {
		t.Fatal("elapsed counter regressed")
	}
	if err != nil {
		if reading.Known {
			t.Fatal("failed observation supplied known time")
		}
		t.Logf("host reports unqualified time: %v", err)
		return
	}
	if !reading.Known || reading.Time.IsZero() || reading.Uncertainty < 0 || reading.Uncertainty > 30*time.Second {
		t.Fatalf("invalid native sample: %+v", reading)
	}
	if reading.Time.Before(wallBefore.Add(-reading.Uncertainty)) || reading.Time.After(wallAfter.Add(reading.Uncertainty)) {
		t.Fatalf("native timestamp disagrees with the host UTC interval: %+v", reading)
	}
	t.Logf("native observation: %s uncertainty=%s", reading.Time.Format(time.RFC3339Nano), reading.Uncertainty)
}

func TestDarwinElapsedAllocations(t *testing.T) {
	if _, known := Elapsed(); !known {
		t.Fatal("elapsed counter unavailable")
	}
	if allocations := testing.AllocsPerRun(100, func() { _, _ = Elapsed() }); allocations != 0 {
		t.Fatalf("elapsed read allocations = %v", allocations)
	}
}
