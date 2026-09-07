package starmap

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestReadOnlyConstructorsStartNoWorkers(t *testing.T) {
	store := storage.NewMemory()
	if err := store.Commit(t.Context(), rootRemoteGeneration(t), ""); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		open func(context.Context) (*Client, error)
	}{
		{"new", func(context.Context) (*Client, error) { return New() }},
		{"context", func(ctx context.Context) (*Client, error) { return NewContext(ctx) }},
		{"durable", func(ctx context.Context) (*Client, error) { return NewContext(ctx, WithCatalogStore(store)) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				before := constructorGoroutines(t)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				client, err := test.open(ctx)
				if err != nil {
					t.Fatal(err)
				}
				initial := client.CurrentCatalogState()
				for _, delay := range []time.Duration{0, 24 * time.Hour} {
					time.Sleep(delay)
					synctest.Wait()
					if extra := addedConstructorGoroutines(before, constructorGoroutines(t)); len(extra) != 0 {
						t.Fatalf("constructor started background workers: %v", extra)
					}
					if current := client.CurrentCatalogState(); current != initial {
						t.Fatal("passive catalog changed without an explicit operation")
					}
				}
			})
		})
	}
}

func TestConstructorWorkerObservationDetectsBlockedAndScheduledWorkers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		before := constructorGoroutines(t)
		if extra := addedConstructorGoroutines(before, constructorGoroutines(t)); len(extra) != 0 {
			t.Fatal("observer reported workers without creation")
		}
		stop := make(chan struct{})
		defer close(stop)
		go func() { <-stop }()
		go func() {
			timer := time.NewTimer(4 * time.Hour)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-stop:
			}
		}()
		synctest.Wait()
		if extra := addedConstructorGoroutines(before, constructorGoroutines(t)); len(extra) != 2 {
			t.Fatalf("observer found %d workers, want two", len(extra))
		}
	})
}

// constructorGoroutines retains stack text so failures identify each new worker.
func constructorGoroutines(t *testing.T) map[string]string {
	t.Helper()
	for size := 64 << 10; size <= 8<<20; size *= 2 {
		buffer := make([]byte, size)
		used := runtime.Stack(buffer, true)
		if used == len(buffer) {
			continue
		}
		result := make(map[string]string)
		for stack := range strings.SplitSeq(strings.TrimSpace(string(buffer[:used])), "\n\n") {
			header, _, _ := strings.Cut(stack, "\n")
			fields := strings.Fields(header)
			if len(fields) < 3 || fields[0] != "goroutine" {
				t.Fatalf("unrecognized goroutine header: %q", header)
			}
			result[fields[1]] = stack
		}
		return result
	}
	t.Fatal("goroutine observation exceeded its buffer limit")
	return nil
}

func addedConstructorGoroutines(before, after map[string]string) []string {
	var added []string
	for id, stack := range after {
		if _, found := before[id]; !found {
			added = append(added, stack)
		}
	}
	return added
}
