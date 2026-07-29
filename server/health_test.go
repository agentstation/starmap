package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
)

func TestHealthSeparatesCatalogFreshnessFromStreamLifecycle(t *testing.T) {
	t.Parallel()

	client, err := starmap.New()
	if err != nil {
		t.Fatalf("starmap.New: %v", err)
	}
	expected := client.CurrentCatalogState()
	srv, err := New(client, DefaultConfig())
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	health := srv.Health()
	if health.State != StateIdle || health.Stream.State != StreamStateIdle {
		t.Fatalf("idle health = %#v", health)
	}
	if health.ActiveGenerationID != expected.GenerationID ||
		!health.CatalogGeneratedAt.Equal(expected.GeneratedAt) {
		t.Fatalf("catalog health = %#v, state = %#v", health, expected)
	}
	if health.CatalogGeneratedAt.IsZero() || health.CatalogAgeSeconds < 0 {
		t.Fatalf("catalog freshness = %#v", health)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	started := srv.Health()
	if started.State != StateServing || started.Stream.State != StreamStateIdle {
		t.Fatalf("started health = %#v", started)
	}
	if !started.CatalogGeneratedAt.Equal(health.CatalogGeneratedAt) {
		t.Fatal("server lifecycle changed catalog freshness")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	stopped := srv.Health()
	if stopped.State != StateStopped || stopped.Stream.State != StreamStateStopped {
		t.Fatalf("stopped health = %#v", stopped)
	}
	if !stopped.CatalogGeneratedAt.Equal(health.CatalogGeneratedAt) {
		t.Fatal("stream shutdown changed catalog freshness")
	}
}

func TestHealthExposesCoalescedPublicationDelivery(t *testing.T) {
	t.Parallel()

	client, err := starmap.New(
		starmap.WithCatalogStore(catalogstore.NewMemory()),
	)
	if err != nil {
		t.Fatalf("starmap.New: %v", err)
	}
	blocked := make(chan struct{})
	started := make(chan struct{})
	var first sync.Once
	client.OnCatalogPublished(func(starmap.CatalogPublishedEvent) error {
		first.Do(func() {
			close(started)
			<-blocked
		})
		return nil
	})
	srv, err := New(client, DefaultConfig())
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	publish := func() {
		t.Helper()
		if _, err := client.Update(
			context.Background(),
			func(
				_ context.Context,
				catalog *catalogs.Catalog,
			) (*starmap.Candidate, error) {
				return starmap.NewCandidate(catalog)
			},
		); err != nil {
			t.Fatalf("Update: %v", err)
		}
	}

	publish()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("blocking publication hook did not start")
	}
	publish()
	publish()
	health := srv.Health()
	if health.Publication.Coalesced != 1 {
		t.Fatalf("publication health = %#v, want one coalesced generation", health.Publication)
	}
	close(blocked)
	deadline := time.Now().Add(time.Second)
	for client.HookStats().Completed < 4 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if stats := client.HookStats(); stats.Completed < 4 {
		t.Fatalf("publication hooks did not drain: %#v", stats)
	}
}
