//go:build linux

package hostclock

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func linuxSample() unix.Timex {
	return unix.Timex{
		Status: unix.STA_NANO, Maxerror: 100, Esterror: 50, Precision: 1,
		Offset: 2, Time: unix.Timeval{Sec: 1_780_000_000, Usec: 123_456_789},
	}
}

func TestLinuxReadingRejectsUnsafeEvidence(t *testing.T) {
	for _, test := range []struct {
		name   string
		state  int
		change func(*unix.Timex)
	}{
		{name: "unsynchronized state", state: unix.TIME_ERROR},
		{name: "pending leap", state: unix.TIME_INS},
		{name: "unknown state", state: 999},
		{name: "unsynchronized flag", change: func(v *unix.Timex) { v.Status |= unix.STA_UNSYNC }},
		{name: "hardware fault", change: func(v *unix.Timex) { v.Status |= unix.STA_CLOCKERR }},
		{name: "pending leap flag", change: func(v *unix.Timex) { v.Status |= unix.STA_DEL }},
		{name: "negative maximum error", change: func(v *unix.Timex) { v.Maxerror = -1 }},
		{name: "negative estimated error", change: func(v *unix.Timex) { v.Esterror = -1 }},
		{name: "negative precision", change: func(v *unix.Timex) { v.Precision = -1 }},
		{name: "maximum error overflow", change: func(v *unix.Timex) { v.Maxerror = math.MaxInt64 }},
		{name: "offset overflow", change: func(v *unix.Timex) { v.Offset = math.MinInt64 }},
		{name: "combined uncertainty", change: func(v *unix.Timex) { v.Maxerror = 30_000_000 }},
		{name: "negative timestamp fraction", change: func(v *unix.Timex) { v.Time.Usec = -1 }},
		{name: "nanosecond overflow", change: func(v *unix.Timex) { v.Time.Usec = 1_000_000_000 }},
		{name: "microsecond overflow", change: func(v *unix.Timex) { v.Status = 0; v.Time.Usec = 1_000_000 }},
		{name: "invalid UTC year", change: func(v *unix.Timex) { v.Time.Sec = math.MaxInt64 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := linuxSample()
			if test.change != nil {
				test.change(&input)
			}
			reading, err := linuxReading(test.state, input)
			if err == nil || reading.Known {
				t.Fatalf("unsafe observation = %+v, %v", reading, err)
			}
		})
	}
}

func TestLinuxReadingPreservesTimestampAndBounds(t *testing.T) {
	for _, nano := range []bool{true, false} {
		input := linuxSample()
		unit := time.Nanosecond
		if !nano {
			input.Status = 0
			input.Time.Usec = 123_456
			unit = time.Microsecond
		}
		input.Offset = -2
		input.Esterror = 200
		reading, err := linuxReading(unix.TIME_OK, input)
		want := time.Unix(input.Time.Sec, input.Time.Usec*int64(unit)).UTC()
		bound := 201*time.Microsecond + 3*unit
		if err != nil || !reading.Known || !reading.Time.Equal(want) || reading.Uncertainty != bound {
			t.Fatalf("nano=%v: observation = %+v, %v; want %v +/- %v", nano, reading, err, want, bound)
		}
	}
}

func TestLinuxObservationUsesReadOnlyQuery(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	calls := 0
	reading, err := observeLinux(ctx, func(v *unix.Timex) (int, error) {
		calls++
		if *v != (unix.Timex{}) {
			t.Fatal("query supplied clock mutation fields")
		}
		*v = linuxSample()
		cancel()
		return unix.TIME_OK, nil
	})
	if calls != 1 || !errors.Is(err, context.Canceled) || reading.Known {
		t.Fatalf("canceled native query = %+v, %v, calls=%d", reading, err, calls)
	}
}

func TestLinuxObservationPropagatesNativeFailure(t *testing.T) {
	reading, err := observeLinux(t.Context(), func(*unix.Timex) (int, error) {
		return -1, unix.EPERM
	})
	if !errors.Is(err, unix.EPERM) || reading.Known {
		t.Fatalf("failed native query = %+v, %v", reading, err)
	}
}

func TestLinuxElapsedRejectsInvalidCounter(t *testing.T) {
	for _, input := range []unix.Timespec{
		{Sec: -1}, {Nsec: -1}, {Nsec: 1_000_000_000}, {Sec: math.MaxInt64},
		{Sec: math.MaxInt64 / int64(time.Second), Nsec: 999_999_999},
	} {
		if _, known := linuxElapsed(input); known {
			t.Fatalf("accepted counter %+v", input)
		}
	}
	if got, known := linuxElapsed(unix.Timespec{Sec: 5, Nsec: 3}); !known || got != 5*time.Second+3 {
		t.Fatalf("valid counter = %v, %v", got, known)
	}
}

func TestLinuxNativeObservation(t *testing.T) {
	wallBefore := time.Now().UTC()
	before, known := Elapsed()
	if !known {
		t.Fatal("CLOCK_BOOTTIME unavailable")
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

func TestLinuxElapsedAllocations(t *testing.T) {
	if _, known := Elapsed(); !known {
		t.Fatal("elapsed counter unavailable")
	}
	if allocations := testing.AllocsPerRun(100, func() { _, _ = Elapsed() }); allocations != 0 {
		t.Fatalf("elapsed read allocations = %v", allocations)
	}
}
