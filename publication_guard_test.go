package starmap

import (
	"context"
	"io/fs"
	"testing"

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
