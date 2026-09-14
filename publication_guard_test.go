package starmap

import (
	"context"
	stderrors "errors"
	"io/fs"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestPublicationGuardsArePassiveAndCumulative(t *testing.T) {
	calls := []int{0, 0, 0}
	client, err := New(WithCatalogStore(storage.NewMemory()),
		WithPublicationGuard(func(context.Context) error { calls[0]++; return nil }),
		WithPublicationGuard(func(context.Context) error { calls[1]++; return fs.ErrPermission }),
		WithPublicationGuard(func(context.Context) error { calls[2]++; return nil }))
	if err != nil {
		t.Fatal(err)
	}
	before := client.CurrentCatalogState()
	_ = client.Catalog()
	if calls[0] != 0 || calls[1] != 0 || calls[2] != 0 {
		t.Fatal("construction or reads called a publication guard")
	}
	called := false
	_, err = client.Update(t.Context(), func(context.Context, *catalogs.Catalog) (*Candidate, error) { called = true; return nil, nil })
	if err != fs.ErrPermission || called {
		t.Fatal("publication guard failed to stop candidate work")
	}
	if _, err := client.Activate(t.Context(), catalogs.Generation{}); err != fs.ErrPermission {
		t.Fatal("activation bypassed publication guards")
	}
	if _, err := client.Rollback(t.Context(), "retained-generation"); err != fs.ErrPermission {
		t.Fatalf("rollback bypassed publication guards: %v", err)
	}
	if _, err := client.Reload(t.Context(), nil); err != fs.ErrPermission {
		t.Fatalf("reload bypassed publication guards: %v", err)
	}
	if calls[0] != 4 || calls[1] != 4 || calls[2] != 0 || client.CurrentCatalogState() != before {
		t.Fatal("guards were overwritten or a refusal changed published state")
	}
}

func TestPublicationGuardPreservesAuthorizedUpdate(t *testing.T) {
	client, err := New(WithCatalogStore(storage.NewMemory()), WithPublicationGuard(func(context.Context) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	called := false
	if _, err := client.Update(nil, func(ctx context.Context, _ *catalogs.Catalog) (*Candidate, error) {
		called = true
		if ctx == nil {
			t.Fatal("missing update context")
		}
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("authorized callback did not run")
	}
	if _, err := New(WithPublicationGuard(nil)); err == nil {
		t.Fatal("nil publication guard succeeded")
	}
}

type queuedPublicationTestKey struct{}

func TestPublicationGuardRechecksQueuedMutation(t *testing.T) {
	for _, operation := range []string{"update", "activate", "reload", "rollback"} {
		t.Run(operation, func(t *testing.T) {
			store := storage.NewMemory()
			target := rootRemoteGeneration(t)
			if err := store.Commit(t.Context(), target, ""); err != nil {
				t.Fatal(err)
			}
			current := target.Copy()
			current.Manifest.GenerationID += "-current"
			if err := store.Commit(t.Context(), current, target.Manifest.GenerationID); err != nil {
				t.Fatal(err)
			}
			var allowed atomic.Bool
			allowed.Store(true)
			queued := make(chan struct{})
			var seen sync.Once
			client, err := New(WithCatalogStore(store), WithPublicationGuard(func(ctx context.Context) error {
				permitted := allowed.Load()
				if ctx.Value(queuedPublicationTestKey{}) == true {
					seen.Do(func() { close(queued) })
				}
				if !permitted {
					return fs.ErrPermission
				}
				return nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			before := client.CurrentCatalogState()
			if operation == "reload" {
				if err := store.Commit(t.Context(), target, current.Manifest.GenerationID); err != nil {
					t.Fatal(err)
				}
			}
			entered, release := make(chan struct{}), make(chan struct{})
			unblock := sync.OnceFunc(func() { close(release) })
			defer unblock()
			blocker := make(chan error, 1)
			go func() {
				_, err := client.Update(t.Context(), func(context.Context, *catalogs.Catalog) (*Candidate, error) {
					close(entered)
					<-release
					return nil, nil
				})
				blocker <- err
			}()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("blocking update did not enter its transaction")
			}
			finished := make(chan error, 1)
			var prepared atomic.Bool
			go func() {
				ctx := context.WithValue(t.Context(), queuedPublicationTestKey{}, true)
				var err error
				switch operation {
				case "update":
					_, err = client.Update(ctx, func(_ context.Context, catalog *catalogs.Catalog) (*Candidate, error) {
						prepared.Store(true)
						return NewCandidate(catalog, CandidateEvidence{})
					})
				case "activate":
					_, err = client.Activate(ctx, target)
				case "reload":
					_, err = client.Reload(ctx, func(catalogs.Generation) error { prepared.Store(true); return nil })
				case "rollback":
					_, err = client.Rollback(ctx, target.Manifest.GenerationID)
				}
				finished <- err
			}()
			select {
			case <-queued:
			case <-time.After(time.Second):
				t.Fatal("queued operation did not check its guard")
			}
			allowed.Store(false)
			unblock()
			if err := <-blocker; err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-finished:
				if !stderrors.Is(err, fs.ErrPermission) {
					t.Fatalf("queued %s returned %v after its guard changed", operation, err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("queued operation did not finish")
			}
			if prepared.Load() || client.CurrentCatalogState() != before {
				t.Fatal("queued mutation bypassed the changed publication guard")
			}
		})
	}
}
