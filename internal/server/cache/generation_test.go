package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCacheGenerationNamespaceCannotRegressOrLeakValues(t *testing.T) {
	cache := New(time.Minute, time.Minute)
	cache.SetGeneration(1, "generation-1", "models", "old")
	if value, found := cache.GetGeneration(1, "generation-1", "models"); !found || value != "old" {
		t.Fatalf("generation-1 value = %#v/%t", value, found)
	}
	cache.ActivateGeneration(2, "generation-2")
	if _, found := cache.GetGeneration(2, "generation-2", "models"); found {
		t.Fatal("generation-2 observed generation-1 cached value")
	}
	cache.SetGeneration(1, "generation-1", "models", "late-old")
	cache.ActivateGeneration(1, "generation-1")
	if cache.GenerationID() != "generation-2" || cache.ItemCount() != 0 {
		t.Fatalf("late old request regressed cache: %#v", cache.GetStats())
	}
	cache.SetGeneration(2, "generation-2", "models", "new")
	if value, found := cache.GetGeneration(2, "generation-2", "models"); !found || value != "new" {
		t.Fatalf("generation-2 value = %#v/%t", value, found)
	}
}

func TestCacheGenerationConcurrentReadersObserveOnlyRequestedNamespace(t *testing.T) {
	cache := New(time.Minute, time.Minute)
	cache.SetGeneration(1, "generation-1", "value", "generation-1")
	var wait sync.WaitGroup
	for sequence := uint64(2); sequence <= 20; sequence++ {
		sequence := sequence
		wait.Go(func() {
			generation := fmt.Sprintf("generation-%d", sequence)
			cache.SetGeneration(sequence, generation, "value", generation)
			if value, found := cache.GetGeneration(sequence, generation, "value"); found && value != generation {
				t.Errorf("generation %d observed %#v", sequence, value)
			}
		})
	}
	wait.Wait()
	if stats := cache.GetStats(); stats.Sequence != 20 || stats.GenerationID != "generation-20" {
		t.Fatalf("final cache state = %#v", stats)
	}
}
