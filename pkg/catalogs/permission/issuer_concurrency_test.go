package permission

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

type issuerTestClock struct {
	mu      sync.Mutex
	reading ClockReading
}

func (c *issuerTestClock) sample() ClockReading {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reading
}

func (c *issuerTestClock) advance(delta time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reading.Time = c.reading.Time.Add(delta)
}

func TestIssuerConcurrentDelayedHeadCannotOverrideNewerObservation(t *testing.T) {
	store := storage.NewMemory()
	first := issuerGeneration(t, "concurrent-first", 1)
	second := issuerGeneration(t, "concurrent-second", 2)
	if err := store.Commit(t.Context(), first, ""); err != nil {
		t.Fatal(err)
	}
	clock := issuerTestClock{reading: ClockReading{Time: time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC), Known: true}}
	ready, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int64
	reader := headReaderFunc(func(ctx context.Context) (catalogs.CatalogAuthorityHead, error) {
		head, err := store.CurrentAuthorityHead(ctx)
		if calls.Add(1) == 1 {
			close(ready)
			select {
			case <-release:
			case <-ctx.Done():
				return catalogs.CatalogAuthorityHead{}, ctx.Err()
			}
		}
		return head, err
	})
	issuer, err := NewIssuer(reader, IssuerConfig{AuthorityID: "enterprise", PolicyID: "production", Clock: clock.sample})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	var group sync.WaitGroup
	t.Cleanup(func() { cancel(); group.Wait() })
	result := make(chan error, 1)
	group.Go(func() { _, err := issuer.ReadPermission(ctx); result <- err })
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := store.Commit(ctx, second, first.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	clock.advance(time.Minute)
	receipt, err := issuer.ReadPermission(ctx)
	if err != nil || receipt.Head != second.Manifest.AuthorityHead {
		t.Fatalf("new receipt=%+v error=%v", receipt, err)
	}
	close(release)
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("delayed old head produced another receipt")
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestIssuerClockFailureRetainsKnownRequirement(t *testing.T) {
	store := storage.NewMemory()
	old := issuerGeneration(t, "older-head", 1)
	newer := issuerGeneration(t, "observed-head", 2)
	if err := store.Commit(t.Context(), old, ""); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(t.Context(), newer, old.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	reading := ClockReading{Time: time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC), Known: true}
	loseClock := true
	reader := headReaderFunc(func(ctx context.Context) (catalogs.CatalogAuthorityHead, error) {
		head, err := store.CurrentAuthorityHead(ctx)
		if loseClock {
			reading.Known = false
		}
		return head, err
	})
	issuer, err := NewIssuer(reader, IssuerConfig{AuthorityID: "enterprise", PolicyID: "production", Clock: func() ClockReading { return reading }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := issuer.ReadPermission(t.Context()); err == nil {
		t.Fatal("lost clock qualification allowed a receipt")
	}
	reading.Known = true
	loseClock = false
	if err := store.Commit(t.Context(), old, newer.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	if got, err := issuer.ReadPermission(t.Context()); err == nil || got != (catalogs.CatalogPermissionEnvelope{}) {
		t.Fatalf("receipt=%+v error=%v", got, err)
	}
}
