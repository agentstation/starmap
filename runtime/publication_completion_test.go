package runtime

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

func TestPublicationCompletionAllowsActiveCaller(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := publicationCompletionContext(t.Context())
		defer cancel()
		time.Sleep(2 * closeJoinTimeout)
		if err := ctx.Err(); err != nil {
			t.Fatalf("active publication completion expired: %v", err)
		}
	})
}

func TestPublicationCompletionBoundsCanceledCaller(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		parent, stop := context.WithCancel(t.Context())
		defer stop()
		ctx, cancel := publicationCompletionContext(parent)
		defer cancel()
		stop()
		synctest.Wait()
		if err := ctx.Err(); err != nil {
			t.Fatalf("cancellation removed completion grace: %v", err)
		}
		time.Sleep(closeJoinTimeout)
		synctest.Wait()
		if ctx.Err() == nil {
			t.Fatal("completion outlived cancellation grace")
		}
	})
}

func TestPublicationCompletionCleanupCancelsWork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		parent, stop := context.WithCancel(t.Context())
		defer stop()
		ctx, cancel := publicationCompletionContext(parent)
		stop()
		synctest.Wait()
		cancel()
		synctest.Wait()
		if ctx.Err() == nil {
			t.Fatal("completion cleanup left work active")
		}
	})
}
