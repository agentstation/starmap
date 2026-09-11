package starmap

import (
	"context"
	"io/fs"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/permission"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

type reloadStore struct {
	storage.Store
	commits atomic.Int64
	read    func(context.Context) (catalogs.Generation, error)
}

func (s *reloadStore) Current(ctx context.Context) (catalogs.Generation, error) {
	if s.read != nil {
		return s.read(ctx)
	}
	return s.Store.Current(ctx)
}

func (s *reloadStore) Commit(context.Context, catalogs.Generation, string) error {
	s.commits.Add(1)
	return fs.ErrPermission
}

func TestReloadActivatesAcceptedGenerationWithoutWriting(t *testing.T) {
	store := storage.NewMemory()
	first := rootRemoteGeneration(t)
	if err := store.Commit(t.Context(), first, ""); err != nil {
		t.Fatal(err)
	}
	watched := &reloadStore{Store: store}
	client, err := New(WithCatalogStore(watched))
	if err != nil {
		t.Fatal(err)
	}
	before := client.CurrentCatalogState()
	next := first.Copy()
	next.Manifest.GenerationID += "-next"
	if err := store.Commit(t.Context(), next, first.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	checked := false
	publication, err := client.Reload(t.Context(), func(g catalogs.Generation) error {
		checked = true
		g.Manifest.GenerationID = "mutated-check-copy"
		g.Payload[0] = 0
		return nil
	})
	if err != nil || !checked || !publication.Published || publication.GenerationID != next.Manifest.GenerationID {
		t.Fatalf("reload = %+v, check=%t, error=%v", publication, checked, err)
	}
	current := client.CurrentCatalogState()
	if current.Sequence != before.Sequence+1 || current.Catalog != before.Catalog || watched.commits.Load() != 0 {
		t.Fatal("reload rewrote storage, replaced equal catalog bytes, or lost its local sequence")
	}
	publication, err = client.Reload(t.Context(), nil)
	if err != nil || publication.Published || client.CurrentCatalogState() != current {
		t.Fatalf("unchanged reload = %+v, %v", publication, err)
	}
}

func TestReloadFailurePreservesAcceptedSnapshot(t *testing.T) {
	for _, mode := range []string{"read", "schema", "payload", "same identity", "check", "canceled check"} {
		t.Run(mode, func(t *testing.T) {
			store := storage.NewMemory()
			first := rootRemoteGeneration(t)
			if err := store.Commit(t.Context(), first, ""); err != nil {
				t.Fatal(err)
			}
			watched := &reloadStore{Store: store}
			client, err := New(WithCatalogStore(watched))
			if err != nil {
				t.Fatal(err)
			}
			before := client.CurrentCatalogState()
			next := first.Copy()
			next.Manifest.GenerationID += "-next"
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var check func(catalogs.Generation) error
			var readErr error
			switch mode {
			case "read":
				readErr = fs.ErrPermission
			case "schema":
				next.Manifest.SchemaVersion++
			case "payload":
				next.Payload[0] = 0
			case "same identity":
				next.Manifest.GenerationID = first.Manifest.GenerationID
				next.Manifest.GeneratedAt = next.Manifest.GeneratedAt.Add(1)
			case "check":
				check = func(catalogs.Generation) error { return fs.ErrPermission }
			case "canceled check":
				check = func(catalogs.Generation) error { cancel(); return nil }
			}
			watched.read = func(context.Context) (catalogs.Generation, error) { return next, readErr }
			if _, err := client.Reload(ctx, check); err == nil {
				t.Fatal("reload accepted a failed read or validation")
			}
			if client.CurrentCatalogState() != before || watched.commits.Load() != 0 {
				t.Fatal("failed reload changed accepted state")
			}
		})
	}
}

func TestReloadRetainsAuthorityIdentityAndSequence(t *testing.T) {
	for _, mode := range []string{"authority", "policy", "replay", "ordinary"} {
		t.Run(mode, func(t *testing.T) {
			input := rootRemoteGeneration(t)
			config := permission.GenerationConfig{AuthorityID: "enterprise", PolicyID: "production", Sequence: 2}
			first, err := permission.PrepareGeneration(input, config)
			if err != nil {
				t.Fatal(err)
			}
			store := storage.NewMemory()
			if err := store.Commit(t.Context(), first, ""); err != nil {
				t.Fatal(err)
			}
			watched := &reloadStore{Store: store}
			client, err := New(WithCatalogStore(watched))
			if err != nil {
				t.Fatal(err)
			}
			before := client.CurrentCatalogState()
			switch mode {
			case "authority":
				config.AuthorityID = "other"
			case "policy":
				config.PolicyID = "other"
			case "replay":
				config.Sequence = 1
			}
			next := input
			if mode != "ordinary" {
				next, err = permission.PrepareGeneration(input, config)
				if err != nil {
					t.Fatal(err)
				}
			}
			watched.read = func(context.Context) (catalogs.Generation, error) { return next, nil }
			if _, err := client.Reload(t.Context(), nil); err == nil {
				t.Fatal("reload changed the selected authority or reversed its sequence")
			}
			if client.CurrentCatalogState() != before || watched.commits.Load() != 0 {
				t.Fatal("refused authority changed active state")
			}
		})
	}
}

func TestReloadOrdersMutationsAndKeepsReadsAvailable(t *testing.T) {
	store := storage.NewMemory()
	first := rootRemoteGeneration(t)
	if err := store.Commit(t.Context(), first, ""); err != nil {
		t.Fatal(err)
	}
	watched := &reloadStore{Store: store}
	attempts := make(chan struct{}, 2)
	client, err := New(WithCatalogStore(watched), WithPublicationGuard(func(context.Context) error { attempts <- struct{}{}; return nil }))
	if err != nil {
		t.Fatal(err)
	}
	before := client.CurrentCatalogState()
	entered, release := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	next := first.Copy()
	next.Manifest.GenerationID += "-next"
	watched.read = func(ctx context.Context) (catalogs.Generation, error) {
		close(entered)
		select {
		case <-release:
			return next, nil
		case <-ctx.Done():
			return catalogs.Generation{}, ctx.Err()
		}
	}
	reloaded := make(chan error, 1)
	go func() { _, err := client.Reload(ctx, nil); reloaded <- err }()
	<-entered
	<-attempts
	updated := make(chan error, 1)
	called := make(chan struct{})
	go func() {
		_, err := client.Update(ctx, func(context.Context, *catalogs.Catalog) (*Candidate, error) {
			close(called)
			return nil, nil
		})
		updated <- err
	}()
	<-attempts
	select {
	case <-called:
		t.Fatal("update crossed an active store reload")
	case <-time.After(20 * time.Millisecond):
	}
	if client.CurrentCatalogState() != before {
		t.Fatal("blocked reload changed the readable snapshot")
	}
	close(release)
	if err := <-reloaded; err != nil {
		t.Fatal(err)
	}
	if err := <-updated; err != nil {
		t.Fatal(err)
	}
	if client.CurrentGenerationID() != next.Manifest.GenerationID || watched.commits.Load() != 0 {
		t.Fatal("reload lost its ordered generation or wrote shared state")
	}
}
