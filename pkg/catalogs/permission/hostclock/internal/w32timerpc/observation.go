package w32timerpc

import (
	"context"
	"time"

	"github.com/oiweiwei/go-msrpc/msrpc/w32t"
)

const observationTimeout = 2 * time.Second

// Windows SCM calls cannot accept a Go context. Keep at most one active native query.
// Cancellation returns to the caller while that query retains its slot until cleanup.
var nativeQuerySlot = make(chan struct{}, 1)

type statusResult struct {
	status *w32t.StatusInfo
	err    error
}

func runStatusQuery(ctx context.Context, observe func(context.Context) (*w32t.StatusInfo, error)) (*w32t.StatusInfo, error) {
	if ctx == nil {
		return nil, invalidReply("a W32Time observation context is required")
	}
	ctx, cancel := context.WithTimeout(ctx, observationTimeout)
	defer cancel()
	select {
	case nativeQuerySlot <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		<-nativeQuerySlot
		return nil, err
	}
	done := make(chan statusResult, 1)
	go func() {
		defer func() { <-nativeQuerySlot }()
		status, err := observe(ctx)
		done <- statusResult{status: status, err: err}
	}()
	select {
	case result := <-done:
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return result.status, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ObserveStatus reads the local W32Time service without changing its settings.
func ObserveStatus(ctx context.Context) (*w32t.StatusInfo, error) {
	return runStatusQuery(ctx, observeLocalStatus)
}
