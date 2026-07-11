package starmap

import (
	"errors"
	"sync"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func consumerCatalog(sm *Client) *catalogs.Catalog {
	catalog := sm.Catalog()
	return catalog
}

func consumerModelDefinition(sm *Client, id string) (catalogs.ModelDefinition, error) {
	catalog := sm.Catalog()
	model, err := catalog.FindModel(id)
	return model, err
}

func TestConsumerCatalogReturnsConcreteImmutableCatalog(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	catalog := consumerCatalog(client)

	if catalog == nil {
		t.Fatal("Client.Catalog returned nil after successful construction")
	}
	if _, ok := any(catalog).(*catalogs.Builder); ok {
		t.Fatal("Client.Catalog exposes mutable Builder")
	}
	if _, ok := any(catalog.Providers()).(interface {
		Set(catalogs.ProviderID, *catalogs.Provider) error
	}); ok {
		t.Fatal("Client.Catalog provider collection exposes Set")
	}
}

func TestConsumerFindModelReturnsCanonicalDefinition(t *testing.T) {
	client, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	model, err := consumerModelDefinition(client, "gpt-4o")
	if err != nil {
		t.Fatalf("FindModel: %v", err)
	}
	if model.ID != "gpt-4o" {
		t.Fatalf("definition ID = %q, want gpt-4o", model.ID)
	}
}

func mustTestCatalog(t testing.TB, reader catalogs.Reader) *catalogs.Catalog {
	t.Helper()
	catalog, err := catalogs.NewCatalog(reader)
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	return catalog
}

func TestPublishedCatalogIsolatedFromReturnedBuilder(t *testing.T) {
	draft := catalogs.NewEmpty()
	if err := draft.SetProvider(catalogs.Provider{ID: "committed", Name: "Committed"}); err != nil {
		t.Fatalf("Seed draft: %v", err)
	}

	c := &Client{catalog: mustTestCatalog(t, catalogs.NewEmpty()), hooks: newHooks()}
	if err := publishTestCatalog(c, draft); err != nil {
		t.Fatalf("Publish draft: %v", err)
	}

	if err := draft.SetProvider(catalogs.Provider{ID: "after-commit", Name: "After Commit"}); err != nil {
		t.Fatalf("Mutate returned builder: %v", err)
	}

	published := c.Catalog()
	if _, found := published.Providers().Get("after-commit"); found {
		t.Fatal("Published generation observed a mutation made after commit")
	}
}

func TestCatalogPublicationIsAtomicForConcurrentReaders(t *testing.T) {
	before := catalogs.NewEmpty()
	if err := before.SetProvider(catalogs.Provider{ID: "before", Name: "Before"}); err != nil {
		t.Fatalf("Seed before generation: %v", err)
	}
	after := catalogs.NewEmpty()
	if err := after.SetProvider(catalogs.Provider{ID: "after", Name: "After"}); err != nil {
		t.Fatalf("Seed after generation: %v", err)
	}

	c := &Client{catalog: mustTestCatalog(t, before), hooks: newHooks()}
	start := make(chan struct{})
	errs := make(chan string, 32)
	var readers sync.WaitGroup
	for range 32 {
		readers.Go(func() {
			<-start
			for range 200 {
				catalog := c.Catalog()
				_, hasBefore := catalog.Providers().Get("before")
				_, hasAfter := catalog.Providers().Get("after")
				if hasBefore == hasAfter {
					errs <- "reader observed neither or both generations"
					return
				}
			}
		})
	}
	close(start)
	if err := publishTestCatalog(c, after); err != nil {
		t.Fatalf("Publish after generation: %v", err)
	}
	readers.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestCatalogPublicationRejectsNilBuilder(t *testing.T) {
	c := &Client{catalog: mustTestCatalog(t, catalogs.NewEmpty()), hooks: newHooks()}
	err := publishTestCatalog(c, nil)
	var validationErr *pkgerrors.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("setCatalog(nil) error = %v, want ValidationError", err)
	}
}

func publishTestCatalog(client *Client, builder *catalogs.Builder) error {
	published, err := snapshotBuilder(builder)
	if err != nil {
		return err
	}
	client.swapCatalogGeneration(published, "")
	return nil
}
