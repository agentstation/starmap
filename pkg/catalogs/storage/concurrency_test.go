package storage

import (
	"context"
	"sync"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

type storePairFactory func(*testing.T) (Store, Store)

func TestCatalogStoreConcurrentSameBaseCAS(t *testing.T) {
	factories := map[string]storePairFactory{
		"memory": func(*testing.T) (Store, Store) {
			store := NewMemory()
			return store, store
		},
		"filesystem": func(t *testing.T) (Store, Store) {
			root := t.TempDir()
			first, err := NewFilesystem(root)
			if err != nil {
				t.Fatalf("NewFilesystem first: %v", err)
			}
			second, err := NewFilesystem(root)
			if err != nil {
				t.Fatalf("NewFilesystem second: %v", err)
			}
			return first, second
		},
		"object": func(t *testing.T) (Store, Store) {
			backend := NewMemoryObjectBackend()
			first, err := NewObject(backend, "concurrent")
			if err != nil {
				t.Fatalf("NewObject first: %v", err)
			}
			second, err := NewObject(backend, "concurrent")
			if err != nil {
				t.Fatalf("NewObject second: %v", err)
			}
			return first, second
		},
	}

	for name, factory := range factories {
		t.Run(name, func(t *testing.T) {
			firstStore, secondStore := factory(t)
			base := testGeneration("cas-base", "base")
			if err := firstStore.Commit(context.Background(), base, ""); err != nil {
				t.Fatalf("Commit base: %v", err)
			}
			candidates := []catalogs.Generation{
				testGeneration("cas-left", "left"),
				testGeneration("cas-right", "right"),
			}
			stores := []Store{firstStore, secondStore}
			start := make(chan struct{})
			results := make(chan error, 2)
			var wait sync.WaitGroup
			for index := range stores {
				wait.Go(func() {
					<-start
					results <- stores[index].Commit(context.Background(), candidates[index], "cas-base")
				})
			}
			close(start)
			wait.Wait()
			close(results)

			var successes, conflicts int
			for err := range results {
				switch {
				case err == nil:
					successes++
				case errors.IsConflict(err):
					conflicts++
				default:
					t.Fatalf("Commit error = %v, want nil or typed conflict", err)
				}
			}
			if successes != 1 || conflicts != 1 {
				t.Fatalf("results: successes=%d conflicts=%d, want 1/1", successes, conflicts)
			}
			current, err := firstStore.Current(context.Background())
			if err != nil {
				t.Fatalf("Current: %v", err)
			}
			if current.Manifest.GenerationID != "cas-left" && current.Manifest.GenerationID != "cas-right" {
				t.Fatalf("current generation = %q", current.Manifest.GenerationID)
			}
		})
	}
}
