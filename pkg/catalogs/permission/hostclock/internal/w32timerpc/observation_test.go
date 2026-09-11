package w32timerpc

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oiweiwei/go-msrpc/msrpc/w32t"
)

func TestNativeCancellationKeepsOneOutstandingQuery(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	releaseQuery := sync.OnceFunc(func() { close(release) })
	t.Cleanup(releaseQuery)
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := runStatusQuery(ctx, func(context.Context) (*w32t.StatusInfo, error) {
			calls.Add(1)
			close(started)
			<-release
			return &w32t.StatusInfo{LCState: 2}, nil
		})
		done <- err
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("native query prevented caller cancellation")
	}
	secondCtx, secondCancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer secondCancel()
	_, err := runStatusQuery(secondCtx, func(context.Context) (*w32t.StatusInfo, error) { calls.Add(1); return nil, nil })
	if !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 1 {
		t.Fatalf("second native query started: %d, %v", calls.Load(), err)
	}
	releaseQuery()
	thirdCtx, thirdCancel := context.WithTimeout(t.Context(), time.Second)
	defer thirdCancel()
	status, err := runStatusQuery(thirdCtx, func(context.Context) (*w32t.StatusInfo, error) {
		calls.Add(1)
		return &w32t.StatusInfo{LCState: 2}, nil
	})
	if err != nil || status == nil || status.LCState != 2 || calls.Load() != 2 {
		t.Fatalf("native query did not recover: %+v, %v", status, err)
	}
}
