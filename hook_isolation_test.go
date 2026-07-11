package starmap

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogstore"
)

func TestHookIsolation(t *testing.T) {
	store := catalogstore.NewMemory()
	client := newPostCommitEventClient(t, store)
	client.hooks.deliverySlots = make(chan struct{}, 1)

	blocked := make(chan struct{})
	started := make(chan struct{})
	client.OnCatalogPublished(func(CatalogPublishedEvent) error {
		close(started)
		<-blocked
		return stderrors.New("hook failure")
	})
	client.OnCatalogPublished(func(CatalogPublishedEvent) error {
		panic("hook panic")
	})

	if err := client.Update(context.Background()); err != nil {
		t.Fatalf("first Update: %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("blocking hook did not start")
	}

	// The single delivery slot is occupied; another committed publication is
	// dropped instead of blocking or spawning unbounded goroutines.
	if err := client.Update(context.Background()); err != nil {
		t.Fatalf("second Update: %v", err)
	}
	stats := client.HookStats()
	if stats.Dropped == 0 {
		t.Fatal("hook saturation was not observable")
	}
	close(blocked)

	deadline := time.Now().Add(time.Second)
	for {
		stats = client.HookStats()
		if stats.Failures >= 2 && stats.Panics >= 1 && stats.Completed >= 1 && stats.MaxLatency > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("hook stats did not converge: %#v", stats)
		}
		time.Sleep(time.Millisecond)
	}
}
