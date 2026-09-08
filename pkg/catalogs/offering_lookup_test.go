package catalogs

import (
	stderrors "errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func offeringLookupCatalog(t testing.TB, count int) *Catalog {
	t.Helper()
	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatal(err)
	}
	if err := builder.SetAuthorModel("author", Model{ID: "definition", Name: "Definition", Authors: []Author{{ID: "author", Name: "Author"}}}); err != nil {
		t.Fatal(err)
	}
	provider := Provider{ID: "provider", Name: "Provider", Aliases: []ProviderID{"alias"}, Models: make(map[string]*Model, count)}
	for i := range count {
		model := testReadViewModel("definition", 1, "standard")
		model.ID = fmt.Sprintf("model-%05d", i)
		provider.Models[model.ID] = model
	}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func TestOfferingLookupAllocationsDoNotScaleWithProviderModels(t *testing.T) {
	small := offeringLookupCatalog(t, 1)
	large := offeringLookupCatalog(t, 1000)
	for _, id := range []ProviderID{"provider", "alias"} {
		t.Run(string(id), func(t *testing.T) {
			measure := func(catalog *Catalog) float64 {
				return testing.AllocsPerRun(20, func() {
					offering, err := catalog.Offering(id, "model-00000")
					if err != nil || offering.ProviderID != "provider" {
						t.Fatalf("lookup: provider=%q, err=%v", offering.ProviderID, err)
					}
				})
			}
			one, thousand := measure(small), measure(large)
			t.Logf("allocations: one model=%.0f, one thousand models=%.0f", one, thousand)
			if thousand > one+1 {
				t.Fatalf("lookup allocations grow with unrelated models: %.0f to %.0f", one, thousand)
			}
		})
	}
}

var offeringLookupResult ProviderOffering

func BenchmarkOfferingLookup(b *testing.B) {
	for _, count := range []int{1, 100, 10000} {
		catalog := offeringLookupCatalog(b, count)
		for _, id := range []ProviderID{"provider", "alias"} {
			b.Run(fmt.Sprintf("models=%d/%s", count, id), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					offering, err := catalog.Offering(id, "model-00000")
					if err != nil {
						b.Fatal(err)
					}
					offeringLookupResult = offering
				}
			})
		}
	}
}

func TestOfferingLookupPreservesCanonicalAliasesAndErrors(t *testing.T) {
	builder, err := NewBuilderFrom(offeringLookupCatalog(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.SetProvider(Provider{ID: "empty", Name: "Empty", Aliases: []ProviderID{"empty-alias"}}); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := catalog.Offering("provider", "model-00000")
	if err != nil {
		t.Fatal(err)
	}
	alias, err := catalog.Offering("alias", "model-00000")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(canonical, alias) {
		t.Fatal("alias lookup changed the canonical offering")
	}
	for _, test := range []struct {
		provider     ProviderID
		model        ProviderModelID
		resource, id string
	}{
		{"unknown", "model-00000", "provider", "unknown"},
		{"provider", "opaque/missing@001", "provider offering", "provider/opaque/missing@001"},
		{"alias", "opaque/missing@001", "provider offering", "provider/opaque/missing@001"},
		{"empty", "missing", "provider offering", "empty/missing"},
		{"empty-alias", "missing", "provider offering", "empty/missing"},
	} {
		t.Run(string(test.provider)+"/"+string(test.model), func(t *testing.T) {
			_, err := catalog.Offering(test.provider, test.model)
			var missing *pkgerrors.NotFoundError
			if !stderrors.As(err, &missing) || missing.Resource != test.resource || missing.ID != test.id {
				t.Fatalf("lookup error = %v; want %s %s", err, test.resource, test.id)
			}
		})
	}
	observation, err := NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	_, err = observation.Offering("alias", "model-00000")
	var missing *pkgerrors.NotFoundError
	if !stderrors.As(err, &missing) || missing.Resource != "provider offering" || missing.ID != "provider/model-00000" {
		t.Fatalf("observation exposed an offering or lost canonical error identity: %v", err)
	}
}

func TestOfferingLookupRetainsCallerOwnershipConcurrently(t *testing.T) {
	catalog := offeringLookupCatalog(t, 100)
	provider, err := catalog.Provider("provider")
	if err != nil {
		t.Fatal(err)
	}
	provider.Aliases[0] = "caller-alias"
	delete(provider.Models, "model-00000")
	var group sync.WaitGroup
	for i := range 8 {
		group.Go(func() {
			id := ProviderID("provider")
			if i%2 != 0 {
				id = "alias"
			}
			for range 64 {
				offering, err := catalog.Offering(id, "model-00000")
				if err != nil {
					t.Error(err)
					return
				}
				if offering.Pricing.Tokens.Input.Per1M != 1 || offering.Limits.ContextWindow != 1000 {
					t.Error("another caller changed retained offering values")
					return
				}
				offering.Pricing.Tokens.Input.Per1M = 999
				offering.Limits.ContextWindow = 999
				body := offering.Modes["fast"].Request.Body["service_tier"]
				if string(body) != `"standard"` {
					t.Errorf("another caller changed request body: %s", body)
					return
				}
				body[0] = '!'
			}
		})
	}
	group.Wait()
	if _, err := catalog.Offering("caller-alias", "model-00000"); err == nil {
		t.Fatal("a caller changed the provider identity index")
	}
}
