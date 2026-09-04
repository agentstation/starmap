package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestSSEFrameWriteDeadlineDefaultsToTwoMinutes proves the per-frame write
// deadline of an unconfigured stream. The broadcaster resets the deadline
// before every frame, so the value bounds one frame and never bounds the
// stream.
func TestSSEFrameWriteDeadlineDefaultsToTwoMinutes(t *testing.T) {
	const want = 2 * time.Minute
	if DefaultWriteTimeout != want {
		t.Fatalf("DefaultWriteTimeout = %s, want %s", DefaultWriteTimeout, want)
	}
	broadcaster := newTestBroadcaster(t, Config{HeartbeatInterval: time.Hour})
	if broadcaster.config.WriteTimeout != want {
		t.Fatalf(
			"configured write timeout = %s, want %s",
			broadcaster.config.WriteTimeout, want,
		)
	}

	writer := newFailingWriter(10)
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(
		http.MethodGet, "/api/v1/updates/stream", nil,
	).WithContext(ctx)
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		broadcaster.ServeHTTP(writer, request)
	}()
	select {
	case <-writer.firstWrite:
	case <-time.After(time.Second):
		t.Fatal("initial SSE frame was not written")
	}
	if err := broadcaster.Publish(Publication{
		GenerationID: "generation-deadline", Sequence: 1,
	}); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	waitForCondition(t, time.Second, "the publication frame set no deadline", func() bool {
		return len(writer.deadlineValues()) >= 3
	})
	cancel()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("SSE handler did not return after caller cancellation")
	}

	// One deadline opens the connection, and one more precedes every frame. So
	// each recorded deadline carries the full per-frame budget.
	deadlines := writer.deadlineValues()
	if len(deadlines) < 3 {
		t.Fatalf("write deadlines = %d, want a connect frame and a publication", len(deadlines))
	}
	for index, deadline := range deadlines {
		remaining := time.Until(deadline)
		if remaining <= want-time.Minute || remaining > want {
			t.Fatalf("deadline %d leaves %s, want about %s", index, remaining, want)
		}
	}
}

// waitForCondition blocks until condition holds. It fails the test otherwise.
func waitForCondition(t *testing.T, within time.Duration, reason string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal(reason)
}
