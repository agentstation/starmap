//go:build windows

package hostclock

import (
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowsNativeElapsed(t *testing.T) {
	query := windows.NewLazySystemDLL("kernel32.dll").NewProc("QueryInterruptTime")
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
