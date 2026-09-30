package catalogs

import (
	"bytes"
	stderrors "errors"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/catalogs/internal/resourcepolicy"
	sourcepayload "github.com/agentstation/starmap/pkg/sources/payload"
)

func reusePayload(t *testing.T, name string) []byte {
	t.Helper()
	builder := NewEmpty()
	if err := builder.SetAuthor(Author{ID: "author", Name: name}); err != nil {
		t.Fatal(err)
	}
	payload, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func mustDecodeForReuse(t *testing.T, payload []byte) *Catalog {
	t.Helper()
	catalog, err := DecodeCatalogPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func TestDecodedCatalogReuseRequiresOpenScopeAndEqualBytes(t *testing.T) {
	payload := reusePayload(t, "Original")
	if mustDecodeForReuse(t, payload) == mustDecodeForReuse(t, payload) {
		t.Fatal("decode without a scope returned a retained catalog")
	}

	release := RetainDecodedCatalogs()
	first := mustDecodeForReuse(t, payload)
	if again := mustDecodeForReuse(t, bytes.Clone(payload)); again != first {
		t.Fatal("equal bytes in one scope decoded again")
	}
	changed := bytes.Replace(payload, []byte("Original"), []byte("Changed!"), 1)
	other := mustDecodeForReuse(t, changed)
	author, err := other.Author("author")
	if other == first || err != nil || author.Name != "Changed!" {
		t.Fatalf("changed bytes returned author %#v, %v; want a full decode", author, err)
	}
	if again := mustDecodeForReuse(t, payload); again != first {
		t.Fatal("changed bytes replaced the retained original catalog")
	}
	encoded, err := EncodeCatalogPayload(first)
	if err != nil || !bytes.Equal(encoded, payload) {
		t.Fatalf("retained catalog differs from its payload: %v", err)
	}

	release()
	if mustDecodeForReuse(t, payload) == first {
		t.Fatal("released scope kept a retained catalog")
	}
	reopened := RetainDecodedCatalogs()
	defer reopened()
	if mustDecodeForReuse(t, payload) == first {
		t.Fatal("new scope returned a catalog from a released scope")
	}
}

func TestDecodedCatalogReuseKeepsCatalogsUntilLastRelease(t *testing.T) {
	payload := reusePayload(t, "Nested")
	outer := RetainDecodedCatalogs()
	inner := RetainDecodedCatalogs()
	first := mustDecodeForReuse(t, payload)
	inner()
	inner()
	if mustDecodeForReuse(t, payload) != first {
		t.Fatal("inner release dropped a catalog that the outer scope retains")
	}
	outer()
	if mustDecodeForReuse(t, payload) == first {
		t.Fatal("last release kept a retained catalog")
	}
}

func TestDecodedCatalogReuseRetainsOnlyCompleteCatalogs(t *testing.T) {
	defer RetainDecodedCatalogs()()
	partial := []byte(`{
		"schema_version": 6,
		"providers": [{"id":"provider","name":"Provider"}],
		"authors": [{"id":"author","name":"Author"}],
		"provider_models": {
			"provider": [
				{"id":"valid","model":"author/valid","name":"Valid"},
				{"id":"invalid","name":"Invalid","limits":{"context_window":"schema-drift","input_tokens":0,"output_tokens":0}}
			]
		},
		"author_models": {
			"author": [
				{"id":"valid","name":"Valid","authors":[{"id":"author","name":"Author"}]}
			]
		},
		"provenance": {}
	}`)
	var diagnostics [2]*Catalog
	for i := range diagnostics {
		catalog, err := DecodeCatalogPayload(partial)
		if _, quarantined := stderrors.AsType[*sourcepayload.QuarantineError](err); !quarantined || catalog == nil {
			t.Fatalf("decode %d = (%v, %v), want a partial diagnostic", i, catalog, err)
		}
		diagnostics[i] = catalog
	}
	if diagnostics[0] == diagnostics[1] {
		t.Fatal("scope retained a partial diagnostic catalog")
	}

	unresolved := []byte(`{
		"schema_version": 6,
		"providers": [{"id":"provider","name":"Provider"}],
		"authors": [],
		"provider_models": {
			"provider": [{"id":"unresolved","name":"Unresolved Provider Record"}]
		},
		"author_models": {},
		"provenance": {}
	}`)
	for i := range 2 {
		observation, err := DecodeSourceObservationPayload(unresolved)
		if err != nil || observation == nil {
			t.Fatalf("observation %d: %v", i, err)
		}
		if catalog, err := DecodeCatalogPayload(unresolved); err == nil || catalog != nil {
			t.Fatalf("decode %d = (%v, %v), want activation failure after an observation", i, catalog, err)
		}
	}
}

func TestDecodedCatalogReuseKeepsGenerationChecks(t *testing.T) {
	manifest := loadGenerationManifestFixture(t)
	manifest.SourceObservations[1].Source = evidence.ProvidersID
	manifest.SchemaVersion = CurrentCatalogSchemaVersion
	manifest.ConsumerCompatibility = ConsumerCompatibility{MinSchemaVersion: CurrentCatalogSchemaVersion, MaxSchemaVersion: CurrentCatalogSchemaVersion}
	scope := testMembershipScope()
	inventory, positive := manifest.SourceObservations[0], manifest.SourceObservations[1]
	scope.Inventory = &MembershipInventory{ObservationID: inventory.ObservationID, ObservedAt: inventory.ObservedAt, ModelIDs: []string{}}
	scope.Additions = []MembershipPresence{{ModelID: "model", ObservationID: positive.ObservationID, ObservedAt: positive.ObservedAt}}
	builder := NewEmpty()
	if err := builder.SetMembershipScopes([]ProviderMembershipScope{scope}); err != nil {
		t.Fatal(err)
	}
	payload, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Payload = DescribeCatalogPayload(payload)
	generation := Generation{Manifest: manifest, Payload: payload}

	defer RetainDecodedCatalogs()()
	first, err := DecodeCatalogGeneration(generation)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := DecodeCatalogGeneration(generation); err != nil || again != first {
		t.Fatalf("equal generation = (%p, %v), want retained catalog %p", again, err, first)
	}

	unbound := generation
	unbound.Manifest.SourceObservations = slices.Clone(manifest.SourceObservations)
	unbound.Manifest.SourceObservations[0].Source = evidence.ModelsDevHTTPID
	if err := unbound.Validate(); err != nil {
		t.Fatalf("invalid fixture: %v", err)
	}
	if catalog, err := DecodeCatalogGeneration(unbound); err == nil || catalog != nil {
		t.Fatal("retained catalog skipped the membership evidence check")
	}

	changed := generation
	changed.Payload = bytes.Clone(payload)
	changed.Payload[len(changed.Payload)-1] = ' '
	if catalog, err := DecodeCatalogGeneration(changed); err == nil || catalog != nil {
		t.Fatal("retained catalog skipped the payload byte check")
	}
}

func TestDecodedCatalogReuseIsBounded(t *testing.T) {
	defer RetainDecodedCatalogs()()
	payloads := make([][]byte, resourcepolicy.MaxRetainedDecodedCatalogs+1)
	catalogs := make([]*Catalog, len(payloads))
	for i := range payloads {
		payloads[i] = reusePayload(t, fmt.Sprint("Author ", i))
	}
	for i := range resourcepolicy.MaxRetainedDecodedCatalogs {
		catalogs[i] = mustDecodeForReuse(t, payloads[i])
	}
	if mustDecodeForReuse(t, payloads[0]) != catalogs[0] {
		t.Fatal("scope dropped a catalog inside its bound")
	}
	last := len(payloads) - 1
	catalogs[last] = mustDecodeForReuse(t, payloads[last])
	if mustDecodeForReuse(t, payloads[0]) != catalogs[0] {
		t.Fatal("scope dropped the most recently used catalog")
	}
	if mustDecodeForReuse(t, payloads[last]) != catalogs[last] {
		t.Fatal("scope did not retain the newest catalog")
	}
	if mustDecodeForReuse(t, payloads[1]) == catalogs[1] {
		t.Fatal("scope exceeded its retained catalog bound")
	}
}

func TestDecodedCatalogReuseConcurrentDecodeReturnsOneCatalog(t *testing.T) {
	payload := reusePayload(t, "Concurrent")
	defer RetainDecodedCatalogs()()
	const readers = 16
	var group sync.WaitGroup
	decoded := make([]*Catalog, readers)
	failures := make([]error, readers)
	for i := range readers {
		group.Go(func() {
			nested := RetainDecodedCatalogs()
			defer nested()
			decoded[i], failures[i] = DecodeCatalogPayload(payload)
		})
	}
	group.Wait()
	for i := range readers {
		if failures[i] != nil {
			t.Fatal(failures[i])
		}
		if decoded[i] != decoded[0] {
			t.Fatal("concurrent decodes of equal bytes returned different catalogs")
		}
	}
	author, err := decoded[0].Author("author")
	if err != nil || author.Name != "Concurrent" {
		t.Fatalf("shared catalog author = %#v, %v", author, err)
	}
}
