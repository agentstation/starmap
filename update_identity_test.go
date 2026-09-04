package starmap

import (
	"context"
	"regexp"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

// uuidShapedIdentity matches the identity that Client.NextID mints.
var uuidShapedIdentity = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// TestUpdateMintsOneIdentityWithoutACandidateIdentity proves that a candidate
// that names no identity keeps the previous behavior of one fresh UUID-shaped
// identity per publication.
func TestUpdateMintsOneIdentityWithoutACandidateIdentity(t *testing.T) {
	store := storage.NewMemory()
	client, err := New(WithCatalogStore(store))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	publication := updateCatalog(t, client, catalogUpdate(func(catalog *catalogs.Builder) error {
		return catalog.SetProvider(catalogs.Provider{ID: "minted", Name: "Minted"})
	}))
	if !publication.Published {
		t.Fatal("the update published no generation")
	}
	if !uuidShapedIdentity.MatchString(publication.GenerationID) {
		t.Fatalf("generation = %q, want one UUID-shaped identity", publication.GenerationID)
	}
	if got := client.CurrentGenerationID(); got != publication.GenerationID {
		t.Fatalf("current generation = %q, want the published identity %q",
			got, publication.GenerationID)
	}
}

// TestUpdateCommitsTheCandidateIdentity proves that a caller that derives the
// identity of its own composition publishes exactly that identity.
func TestUpdateCommitsTheCandidateIdentity(t *testing.T) {
	const derived = "generation-upstream.local.0123456789ab"
	store := storage.NewMemory()
	client, err := New(WithCatalogStore(store))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	publication, err := client.Update(context.Background(), func(
		_ context.Context,
		current *catalogs.Catalog,
	) (*Candidate, error) {
		builder, err := catalogs.NewBuilderFrom(current)
		if err != nil {
			return nil, err
		}
		if err := builder.SetProvider(catalogs.Provider{ID: "derived", Name: "Derived"}); err != nil {
			return nil, err
		}
		catalog, err := builder.Build()
		if err != nil {
			return nil, err
		}
		return NewCandidate(catalog, CandidateEvidence{}, WithCandidateGenerationID(derived))
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !publication.Published {
		t.Fatal("the update published no generation")
	}
	if publication.GenerationID != derived {
		t.Fatalf("generation = %q, want the derived identity %q", publication.GenerationID, derived)
	}
	if got := client.CurrentGenerationID(); got != derived {
		t.Fatalf("current generation = %q, want the derived identity %q", got, derived)
	}
	current, err := store.Current(context.Background())
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if got := current.Manifest.GenerationID; got != derived {
		t.Fatalf("committed generation = %q, want the derived identity %q", got, derived)
	}
}

// TestCandidateIdentityRejectsAnEmptyValue proves that the option names one
// identity. A caller that has no identity omits the option.
func TestCandidateIdentityRejectsAnEmptyValue(t *testing.T) {
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if _, err := NewCandidate(catalog, CandidateEvidence{}, WithCandidateGenerationID("  ")); err == nil {
		t.Fatal("NewCandidate accepted a blank generation ID")
	}
}
