//go:build windows

package hostclock

import (
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowsNativeElapsed(t *testing.T) {
	query := windows.NewLazySystemDLL("api-ms-win-core-realtime-l1-1-1.dll").NewProc("QueryInterruptTime")
	if err := query.Find(); err != nil {
		t.Fatal(err)
	}
	readInterrupt := func() uint64 {
		var ticks uint64
		_, _, _ = query.Call(uintptr(unsafe.Pointer(&ticks)))
		return ticks
	}
	firstLower := readInterrupt()
	first, known := Elapsed()
	firstUpper := readInterrupt()
	if !known || first < 0 {
		t.Fatal("elapsed counter unavailable")
	}
	time.Sleep(20 * time.Millisecond)
	secondLower := readInterrupt()
	second, known := Elapsed()
	secondUpper := readInterrupt()
	if !known || second <= first || secondLower < firstUpper {
		t.Fatal("elapsed counter did not advance")
	}
	// Each runtime sample must fall between the corresponding native samples.
	minimum := time.Duration(secondLower-firstUpper) * 100 * time.Nanosecond
	maximum := time.Duration(secondUpper-firstLower) * 100 * time.Nanosecond
	if delta := second - first; delta < minimum || delta > maximum {
		t.Fatalf("runtime interval %s outside native interval [%s, %s]", delta, minimum, maximum)
	}
}

func TestWindowsElapsedAllocations(t *testing.T) {
	if _, known := Elapsed(); !known {
		t.Fatal("elapsed counter unavailable")
	}
	if allocations := testing.AllocsPerRun(100, func() { _, _ = Elapsed() }); allocations != 0 {
		t.Fatalf("elapsed read allocations = %v", allocations)
	}
}

func TestWindowsObservationRemainsUnqualified(t *testing.T) {
	reading, err := Observe(t.Context())
	if err == nil || reading.Known {
		t.Fatalf("unqualified Windows observation = %+v, %v", reading, err)
	}
}

func TestWindowsNativeObservationWithProfile(t *testing.T) {
	observer, err := NewWindowsObserver(windowsTestProfile())
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC()
	reading, err := observer.Observe(t.Context())
	after := time.Now().UTC()
	if err != nil {
		if reading.Known {
			t.Fatal("failed Windows observation returned known time")
		}
		t.Logf("Windows source is unqualified for the test profile: %v", err)
		return
	}
	if !reading.Known || reading.Uncertainty <= 0 || reading.Uncertainty > 30*time.Second {
		t.Fatalf("invalid Windows reading: %+v", reading)
	}
	if reading.Time.Before(before.Add(-reading.Uncertainty)) || reading.Time.After(after.Add(reading.Uncertainty)) {
		t.Fatalf("Windows status timestamp disagrees with the host UTC interval: %+v", reading)
	}
	t.Logf("Windows observation: %s uncertainty=%s", reading.Time.Format(time.RFC3339Nano), reading.Uncertainty)
}
