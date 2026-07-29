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

	blocked := make(chan struct{})
	started := make(chan struct{})
	delivered := make(chan uint64, 3)
	first := true
	client.OnCatalogPublished(func(event CatalogPublishedEvent) error {
		delivered <- event.Sequence
		if first {
			first = false
			close(started)
			<-blocked
		}
		return stderrors.New("hook failure")
	})
	client.OnCatalogPublished(func(CatalogPublishedEvent) error {
		panic("hook panic")
	})

	if _, err := client.Update(context.Background(), postCommitTestUpdate); err != nil {
		t.Fatalf("first Update: %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("blocking hook did not start")
	}
	if sequence := <-delivered; sequence != 2 {
		t.Fatalf("first delivered sequence = %d, want 2", sequence)
	}

	// While the first generation is blocked, one pending generation is retained
	// and later publications coalesce toward the newest sequence.
	if _, err := client.Update(context.Background(), postCommitTestUpdate); err != nil {
		t.Fatalf("second Update: %v", err)
	}
	if _, err := client.Update(context.Background(), postCommitTestUpdate); err != nil {
		t.Fatalf("third Update: %v", err)
	}
	stats := client.HookStats()
	if stats.Coalesced != 1 {
		t.Fatalf("coalesced generations = %d, want 1", stats.Coalesced)
	}
	close(blocked)

	deadline := time.Now().Add(time.Second)
	for {
		stats = client.HookStats()
		if stats.Failures >= 4 && stats.Panics >= 2 && stats.Completed >= 4 && stats.MaxLatency > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("hook stats did not converge: %#v", stats)
		}
		time.Sleep(time.Millisecond)
	}
	if sequence := <-delivered; sequence != 4 {
		t.Fatalf("coalesced delivered sequence = %d, want newest 4", sequence)
	}
	select {
	case sequence := <-delivered:
		t.Fatalf("unexpected intermediate delivered sequence %d", sequence)
	default:
	}
}
