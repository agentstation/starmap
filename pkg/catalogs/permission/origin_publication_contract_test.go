package permission

import (
	"context"
	stderrors "errors"
	"math"
	"sync"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestOriginPublicationRequiresExplicitBootstrap(t *testing.T) {
	store := storage.NewMemory()
	input := originInput(t, true)
	if err := store.Commit(t.Context(), input, ""); err != nil {
		t.Fatal(err)
	}
	publisher := newTestPublisher(t, store)
	if _, err := publisher.PublishCatalog(t.Context(), input, input.Manifest.GenerationID); err == nil {
		t.Fatal("ordinary store became an authority without bootstrap")
	}
	first, err := publisher.BootstrapCatalog(t.Context(), input, input.Manifest.GenerationID)
	if err != nil || first.Manifest.AuthorityHead.Sequence != 1 {
		t.Fatalf("bootstrap: %+v / %v", first.Manifest.AuthorityHead, err)
	}
	retry, err := publisher.BootstrapCatalog(t.Context(), input, input.Manifest.GenerationID)
	if err != nil || retry.Manifest.GenerationID != first.Manifest.GenerationID {
		t.Fatalf("bootstrap retry: %s / %v", retry.Manifest.GenerationID, err)
	}
	if _, err := publisher.BootstrapCatalog(t.Context(), originInput(t, false), first.Manifest.GenerationID); err == nil {
		t.Fatal("bootstrap reset an established authority")
	}
	if _, err := publisher.PublishCatalog(t.Context(), first, first.Manifest.GenerationID); err == nil {
		t.Fatal("origin relabeled an existing authority generation")
	}
}

func TestOriginPublicationRefusesInvalidPredecessor(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		change func(*catalogs.Generation)
	}{
		{"different authority", func(g *catalogs.Generation) { g.Manifest.AuthorityHead.AuthorityID = "different" }},
		{"different policy", func(g *catalogs.Generation) { g.Manifest.AuthorityHead.PolicyID = "different" }},
		{"unsupported permission", func(g *catalogs.Generation) { g.Manifest.AuthorityHead.PermissionSchemaVersion++ }},
		{"exhausted sequence", func(g *catalogs.Generation) { g.Manifest.AuthorityHead.Sequence = math.MaxUint64 }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			store := storage.NewMemory()
			current := issuerGeneration(t, "current", 1)
			scenario.change(&current)
			if err := store.Commit(t.Context(), current, ""); err != nil {
				t.Fatal(err)
			}
			publisher := newTestPublisher(t, store)
			if _, err := publisher.PublishCatalog(t.Context(), originInput(t, true), current.Manifest.GenerationID); err == nil {
				t.Fatal("origin replaced an invalid durable predecessor")
			}
			got, err := store.Current(t.Context())
			if err != nil || got.Manifest.AuthorityHead != current.Manifest.AuthorityHead {
				t.Fatalf("refusal changed the stored authority: %+v / %v", got.Manifest.AuthorityHead, err)
			}
		})
	}
}

func TestOriginPublicationRefusesCanceledAndUnreadableState(t *testing.T) {
	input := originInput(t, true)
	readFailure := &errors.ConfigError{Component: "test store", Message: "unreadable predecessor"}
	reads := 0
	store := publicationCurrentHook{Store: storage.NewMemory(), read: func(context.Context) (catalogs.Generation, error) {
		reads++
		return catalogs.Generation{}, readFailure
	}}
	publisher := newTestPublisher(t, store)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := publisher.PublishCatalog(ctx, input, ""); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("canceled publication: %v", err)
	}
	if _, err := publisher.PublishCatalog(t.Context(), catalogs.Generation{}, ""); err == nil || reads != 0 {
		t.Fatal("invalid input or canceled call reached storage")
	}
	if _, err := publisher.PublishCatalog(t.Context(), input, ""); !stderrors.Is(err, readFailure) {
		t.Fatalf("unreadable store permitted a sequence reset: %v", err)
	}
	if _, err := store.Store.Current(t.Context()); !errors.IsNotFound(err) {
		t.Fatalf("failed predecessor read changed the store: %v", err)
	}
	for _, invalid := range []*Publisher{nil, {}} {
		if _, err := invalid.PublishCatalog(t.Context(), input, ""); err == nil {
			t.Fatal("uninitialized publisher accepted a catalog")
		}
	}
	if _, err := publisher.PublishCatalog(nil, input, ""); err == nil {
		t.Fatal("publisher accepted a nil context")
	}
}

func TestOriginPublicationConcurrentWritersPreserveOneSuccessor(t *testing.T) {
	store := storage.NewMemory()
	publisher := newTestPublisher(t, store)
	input := originInput(t, true)
	first, err := publisher.PublishCatalog(t.Context(), input, "")
	if err != nil {
		t.Fatal(err)
	}
	left, right := input.Copy(), input.Copy()
	left.Manifest.SyncRunID, right.Manifest.SyncRunID = "left", "right"
	outcomes := make(chan error, 2)
	var wg sync.WaitGroup
	for _, candidate := range []catalogs.Generation{left, right} {
		wg.Go(func() {
			_, err := publisher.PublishCatalog(t.Context(), candidate, first.Manifest.GenerationID)
			outcomes <- err
		})
	}
	wg.Wait()
	close(outcomes)
	var passed, conflicts int
	for err := range outcomes {
		switch {
		case err == nil:
			passed++
		case errors.IsConflict(err):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent result: %v", err)
		}
	}
	if passed != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", passed, conflicts)
	}
	current, err := store.Current(t.Context())
	if err != nil || current.Manifest.AuthorityHead.Sequence != 2 {
		t.Fatalf("concurrent publication skipped or reset its sequence: %+v / %v", current.Manifest.AuthorityHead, err)
	}
}
