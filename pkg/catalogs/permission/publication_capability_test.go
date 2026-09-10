package permission

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestPublisherRequiresIndependentReadCapability(t *testing.T) {
	store := storage.NewMemory()
	current := issuerGeneration(t, "capability-publication", 1)
	if err := store.Commit(t.Context(), current, ""); err != nil {
		t.Fatal(err)
	}
	ordinary := struct{ storage.Store }{store}
	publisher := newTestPublisher(t, ordinary)
	if _, err := publisher.Current(t.Context()); err != nil {
		t.Fatal(err)
	}
	if head, err := publisher.CurrentAuthorityHead(t.Context()); err == nil || head != (catalogs.CatalogAuthorityHead{}) {
		t.Fatalf("ordinary reads granted a current observation: head=%+v error=%v", head, err)
	}
	other, err := NewPublisher(store, PublisherConfig{AuthorityID: "different", PolicyID: "production"})
	if err != nil {
		t.Fatal(err)
	}
	if head, err := other.CurrentAuthorityHead(t.Context()); err == nil || head != (catalogs.CatalogAuthorityHead{}) {
		t.Fatalf("different authority granted an observation: head=%+v error=%v", head, err)
	}
}

func TestPublisherConstructorAndCanceledCallsArePassive(t *testing.T) {
	calls := 0
	store := publicationCurrentHook{Store: storage.NewMemory(), read: func(context.Context) (catalogs.Generation, error) {
		calls++
		return catalogs.Generation{}, nil
	}}
	publisher := newTestPublisher(t, store)
	for _, config := range []PublisherConfig{{}, {AuthorityID: "enterprise"}, {PolicyID: "production"}} {
		if _, err := NewPublisher(store, config); err == nil {
			t.Fatal("accepted missing publication identity")
		}
	}
	config := PublisherConfig{AuthorityID: "enterprise", PolicyID: "production"}
	if _, err := NewPublisher(nil, config); err == nil {
		t.Fatal("accepted nil store")
	}
	var absent *storage.Memory
	if _, err := NewPublisher(absent, config); err == nil {
		t.Fatal("accepted typed nil store")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := publisher.Commit(ctx, issuerGeneration(t, "canceled-publication", 1), ""); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if _, err := publisher.Current(ctx); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if _, err := publisher.Get(ctx, "canceled-publication"); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if _, err := publisher.CurrentAuthorityHead(ctx); !stderrors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if calls != 0 {
		t.Fatal("constructor or canceled call read storage")
	}
	for _, invalid := range []*Publisher{nil, {}} {
		if err := invalid.Commit(t.Context(), catalogs.Generation{}, ""); err == nil {
			t.Fatal("uninitialized publisher accepted a commit")
		}
	}
	if _, err := publisher.Current(nil); err == nil {
		t.Fatal("accepted nil context")
	}
}
