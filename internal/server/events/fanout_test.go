package events

import (
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/rs/zerolog"
)

func TestFanoutSkipPolicy(t *testing.T) {
	logger := zerolog.Nop()
	fanout := NewFanout[string](BackpressureSkip, &logger)

	var closed atomic.Bool
	result := fanout.Deliver([]DeliveryTarget[string]{
		{
			ID: "ready",
			Send: func(string) error {
				return nil
			},
		},
		{
			ID: "full",
			Send: func(string) error {
				return ErrBackpressure
			},
			Close: func() error {
				closed.Store(true)
				return nil
			},
		},
	}, "event")

	if result.Sent != 1 {
		t.Fatalf("sent = %d, want 1", result.Sent)
	}
	if result.Skipped != 1 {
		t.Fatalf("skipped = %d, want 1", result.Skipped)
	}
	if result.Disconnected != 0 {
		t.Fatalf("disconnected = %d, want 0", result.Disconnected)
	}
	if closed.Load() {
		t.Fatal("skip policy closed backpressured target")
	}

	stats := fanout.Stats()
	if stats.Sent != 1 || stats.Skipped != 1 || stats.Disconnected != 0 || stats.Failed != 0 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestFanoutDisconnectPolicy(t *testing.T) {
	logger := zerolog.Nop()
	fanout := NewFanout[string](BackpressureDisconnect, &logger)

	var closed atomic.Bool
	result := fanout.Deliver([]DeliveryTarget[string]{
		{
			ID: "full",
			Send: func(string) error {
				return ErrBackpressure
			},
			Close: func() error {
				closed.Store(true)
				return nil
			},
		},
	}, "event")

	if result.Disconnected != 1 {
		t.Fatalf("disconnected = %d, want 1", result.Disconnected)
	}
	if !closed.Load() {
		t.Fatal("disconnect policy did not close backpressured target")
	}

	stats := fanout.Stats()
	if stats.Disconnected != 1 {
		t.Fatalf("stats disconnected = %d, want 1", stats.Disconnected)
	}
}

func TestTrySendReportsBackpressure(t *testing.T) {
	ch := make(chan string, 1)
	if err := TrySend(ch, "first"); err != nil {
		t.Fatalf("TrySend returned unexpected error: %v", err)
	}
	if err := TrySend(ch, "second"); err != ErrBackpressure {
		t.Fatalf("TrySend error = %v, want ErrBackpressure", err)
	}
}

func TestFanoutDoesNotStartPerTargetGoroutines(t *testing.T) {
	logger := zerolog.Nop()
	fanout := NewFanout[int](BackpressureSkip, &logger)
	targets := make([]DeliveryTarget[int], 1000)
	for i := range targets {
		targets[i] = DeliveryTarget[int]{
			ID: "target",
			Send: func(int) error {
				return nil
			},
		}
	}

	before := runtime.NumGoroutine()
	result := fanout.Deliver(targets, 1)
	after := runtime.NumGoroutine()

	if result.Sent != len(targets) {
		t.Fatalf("sent = %d, want %d", result.Sent, len(targets))
	}
	if after > before+2 {
		t.Fatalf("fanout appears to have started goroutines: before=%d after=%d", before, after)
	}
}
