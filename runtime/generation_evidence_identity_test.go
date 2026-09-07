package runtime

import (
	"reflect"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestReceiptOnlyUpdatePublishesDistinctGeneration(t *testing.T) {
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	older := scopedProviderLayer(t, "older", "1", at)
	newer := scopedProviderLayer(t, "newer", "1", at.Add(time.Hour))
	store := storage.NewMemory()
	connected := openTestRuntime(t, WithSource(testReviewedDefinitionsSource(t, []ProviderLayer{older, newer})),
		WithClientOptions(starmap.WithCatalogStore(store)), WithProviderBindings(*older.Receipt.ProviderBinding, *newer.Receipt.ProviderBinding))
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := connected.publishProviders(t.Context(), []ProviderLayer{older, newer}, connected.lease.epoch()); err != nil {
		t.Fatal(err)
	}
	before, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	updated := scopedProviderLayer(t, "older", "1", at.Add(time.Minute))
	if _, err := connected.publishProviders(t.Context(), []ProviderLayer{updated}, connected.lease.epoch()); err != nil {
		t.Fatal(err)
	}
	after, err := store.Current(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if before.Manifest.Payload.Checksum != after.Manifest.Payload.Checksum {
		t.Fatal("fixture did not isolate a receipt-only update")
	}
	if before.Manifest.GenerationID == after.Manifest.GenerationID {
		t.Fatal("receipt-only update reused the previous immutable generation")
	}
	found := false
	for _, link := range after.Manifest.SourceObservations {
		if link.ObservationID == older.Receipt.Link.ObservationID {
			t.Fatal("new generation retained the replaced receipt")
		}
		found = found || link.ObservationID == updated.Receipt.Link.ObservationID
	}
	if !found {
		t.Fatal("new generation omitted the current receipt")
	}
}

func TestEffectiveIdentityBindsCanonicalPublicationEvidence(t *testing.T) {
	at := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	one := scopedProviderLayer(t, "one", "1", at).Receipt.Link
	two := scopedProviderLayer(t, "two", "1", at).Receipt.Link
	input := starmap.CandidateEvidence{SourceObservations: []catalogs.SourceObservationLink{two, one}}
	original := append([]catalogs.SourceObservationLink(nil), input.SourceObservations...)
	checksum := func(payload string, candidate starmap.CandidateEvidence) string {
		t.Helper()
		result, err := effectiveEvidenceChecksum(payload, candidate)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	baseline := checksum("payload-one", input)
	if !reflect.DeepEqual(input.SourceObservations, original) {
		t.Fatal("identity construction changed caller-owned evidence")
	}
	reordered := starmap.CandidateEvidence{SourceObservations: []catalogs.SourceObservationLink{one, two}, ReviewCandidates: []evidence.ReviewCandidate{}}
	if checksum("payload-one", reordered) != baseline {
		t.Fatal("equivalent evidence changed the generation identity")
	}
	if checksum("payload-two", input) == baseline {
		t.Fatal("identity omitted catalog bytes")
	}
	replaced := starmap.CandidateEvidence{SourceObservations: []catalogs.SourceObservationLink{one}}
	if checksum("payload-one", replaced) == baseline {
		t.Fatal("identity omitted receipt membership")
	}
	review := evidence.ReviewCandidate{Code: evidence.ReviewCandidateUnresolvedModelReference, ProviderID: "provider", ProviderModelID: "model",
		SourceID: one.Source, SourceObservationID: one.ObservationID, SourceRevision: one.Revision, EvidenceChecksum: one.EvidenceChecksum, Reason: "model needs review"}
	input.ReviewCandidates = []evidence.ReviewCandidate{review}
	reviewed := checksum("payload-one", input)
	if reviewed == baseline {
		t.Fatal("identity omitted review evidence")
	}
	input.ReviewCandidates[0].Reason = "model link was removed"
	if checksum("payload-one", input) == reviewed {
		t.Fatal("identity omitted review details")
	}
	if checksum("payload-one", starmap.CandidateEvidence{}) != "payload-one" {
		t.Fatal("empty evidence changed the baseline identity input")
	}
}
