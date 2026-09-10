package permission

import (
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func newTestPublisher(t *testing.T, store storage.Store) *Publisher {
	t.Helper()
	publisher, err := NewPublisher(store, PublisherConfig{AuthorityID: "enterprise", PolicyID: "production"})
	if err != nil {
		t.Fatal(err)
	}
	return publisher
}

func TestPublisherRefusesAuthorityRollbackAfterReopen(t *testing.T) {
	root := filepath.Join(t.TempDir(), "catalog-store")
	store, err := storage.NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	current := issuerGeneration(t, "current-publication", 2)
	publisher := newTestPublisher(t, store)
	if err := publisher.Commit(t.Context(), current, ""); err != nil {
		t.Fatal(err)
	}
	reopened, err := storage.NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	publisher = newTestPublisher(t, reopened)
	older := issuerGeneration(t, "older-publication", 1)
	if err := publisher.Commit(t.Context(), older, current.Manifest.GenerationID); err == nil {
		t.Error("accepted an older permission sequence after reopen")
	}
	got, err := reopened.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Manifest.AuthorityHead != current.Manifest.AuthorityHead {
		t.Error("rollback replaced the durable authority requirement")
	}
}

func ordinaryPublication(t *testing.T, id string) catalogs.Generation {
	t.Helper()
	generation := issuerGeneration(t, id, 1)
	generation.Manifest.ManifestVersion = catalogs.CurrentGenerationManifestVersion
	generation.Manifest.AuthorityHead = catalogs.CatalogAuthorityHead{}
	if err := generation.Validate(); err != nil {
		t.Fatal(err)
	}
	return generation
}

func TestPublisherBootstrapAndIdempotency(t *testing.T) {
	store := storage.NewMemory()
	ordinary := ordinaryPublication(t, "ordinary-seed")
	if err := store.Commit(t.Context(), ordinary, ""); err != nil {
		t.Fatal(err)
	}
	publisher := newTestPublisher(t, store)
	first := issuerGeneration(t, "first-authority", 1)
	if err := publisher.Commit(t.Context(), first, ordinary.Manifest.GenerationID); err == nil {
		t.Fatal("ordinary store acquired authority without explicit bootstrap")
	}
	if err := publisher.Bootstrap(t.Context(), first, "wrong-predecessor"); err == nil {
		t.Fatal("bootstrap ignored its predecessor")
	}
	if err := publisher.Bootstrap(t.Context(), first, ordinary.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	if err := publisher.Bootstrap(t.Context(), first, ordinary.Manifest.GenerationID); err != nil {
		t.Fatalf("bootstrap retry: %v", err)
	}
	second := issuerGeneration(t, "second-authority", 2)
	if err := publisher.Bootstrap(t.Context(), second, first.Manifest.GenerationID); err == nil {
		t.Fatal("bootstrap replaced an established authority")
	}
	if err := publisher.Commit(t.Context(), second, first.Manifest.GenerationID); err != nil {
		t.Fatal(err)
	}
	if err := publisher.Commit(t.Context(), second, first.Manifest.GenerationID); err != nil {
		t.Fatalf("commit retry: %v", err)
	}
	changed := second
	changed.Manifest.SyncRunID = "changed-under-existing-identity"
	if err := publisher.Commit(t.Context(), changed, first.Manifest.GenerationID); err == nil {
		t.Fatal("idempotent retry allowed different immutable manifest bytes")
	}
	if err := publisher.Commit(t.Context(), ordinary, second.Manifest.GenerationID); err == nil {
		t.Fatal("ordinary generation replaced the authority")
	}
	got, err := publisher.Get(t.Context(), second.Manifest.GenerationID)
	if err != nil || got.Manifest.SyncRunID != second.Manifest.SyncRunID {
		t.Fatalf("generation=%+v error=%v", got.Manifest, err)
	}
}

func TestPublisherRefusesChangedIdentityAndSemantics(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		change func(*catalogs.Generation)
	}{
		{"authority", func(g *catalogs.Generation) { g.Manifest.AuthorityHead.AuthorityID = "another" }},
		{"policy", func(g *catalogs.Generation) { g.Manifest.AuthorityHead.PolicyID = "another" }},
		{"same sequence", func(g *catalogs.Generation) { g.Manifest.AuthorityHead.Sequence = 1 }},
		{"future semantics", func(g *catalogs.Generation) { g.Manifest.AuthorityHead.PermissionSchemaVersion++ }},
		{"invalid payload", func(g *catalogs.Generation) { g.Payload = append(g.Payload, ' ') }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			store := storage.NewMemory()
			publisher := newTestPublisher(t, store)
			current := issuerGeneration(t, "selected-publication", 1)
			if err := publisher.Commit(t.Context(), current, ""); err != nil {
				t.Fatal(err)
			}
			next := issuerGeneration(t, "candidate-publication", 2)
			scenario.change(&next)
			if err := publisher.Commit(t.Context(), next, current.Manifest.GenerationID); err == nil {
				t.Fatal("invalid publication replaced the authority")
			}
			got, err := publisher.Current(t.Context())
			if err != nil || got.Manifest.AuthorityHead != current.Manifest.AuthorityHead {
				t.Fatalf("head=%+v error=%v", got.Manifest.AuthorityHead, err)
			}
		})
	}
}

func TestPublisherRefusesDowngradeButExposesFutureHead(t *testing.T) {
	store := storage.NewMemory()
	current := issuerGeneration(t, "future-permissions", 5)
	current.Manifest.AuthorityHead.PermissionSchemaVersion++
	if err := store.Commit(t.Context(), current, ""); err != nil {
		t.Fatal(err)
	}
	publisher := newTestPublisher(t, store)
	head, err := publisher.CurrentAuthorityHead(t.Context())
	if err != nil || head != current.Manifest.AuthorityHead {
		t.Fatalf("head=%+v error=%v", head, err)
	}
	next := issuerGeneration(t, "older-publisher-proposal", 6)
	if err := publisher.Commit(t.Context(), next, current.Manifest.GenerationID); err == nil {
		t.Fatal("older publisher silently downgraded unknown permission semantics")
	}
	issuer, err := NewIssuer(publisher, IssuerConfig{AuthorityID: "enterprise", PolicyID: "production", Clock: func() ClockReading {
		return ClockReading{Time: current.Manifest.GeneratedAt, Known: true}
	}})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := issuer.ReadPermission(t.Context())
	if err != nil || receipt.Head != head || receipt.Head.SupportsPermissions() {
		t.Fatalf("receipt=%+v error=%v", receipt, err)
	}
}
