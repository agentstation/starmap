package artifact

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
)

func reviewReceiptFixture(t *testing.T) PublicationReceipt {
	t.Helper()
	receipt := publicationReceiptFixture(t)
	link := *receipt.Sources[0].Observation
	receipt.Reviews = []evidence.ReviewCandidate{{Code: evidence.ReviewCandidateUnresolvedModelReference,
		ProviderID: "provider", ProviderModelID: "unresolved", SourceID: link.Source,
		SourceObservationID: link.ObservationID, SourceRevision: link.Revision, EvidenceChecksum: link.EvidenceChecksum,
		Reason: "requires a reviewed canonical model reference"}}
	receipt.ReviewObservations = []catalogs.SourceObservationLink{link}
	return receipt
}

func TestPublicationReceiptCarriesReviewsWithOriginalEvidence(t *testing.T) {
	receipt := reviewReceiptFixture(t)
	data, err := EncodePublicationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := VerifyPublicationReceipt(data, checksum(data), receipt.Artifact)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Reviews) != 1 || restored.Reviews[0] != receipt.Reviews[0] || restored.ReviewObservations[0] != receipt.ReviewObservations[0] {
		t.Fatal("receipt changed review evidence")
	}
	for _, scenario := range []string{"missing link", "different digest", "future link", "unreferenced link", "duplicate review", "duplicate link"} {
		t.Run(scenario, func(t *testing.T) {
			input := reviewReceiptFixture(t)
			switch scenario {
			case "missing link":
				input.ReviewObservations = nil
			case "different digest":
				input.Reviews[0].EvidenceChecksum = "sha256:" + strings.Repeat("b", 64)
			case "future link":
				input.ReviewObservations[0].ObservedAt = input.CompletedAt.Add(time.Second)
			case "unreferenced link":
				input.Reviews = nil
			case "duplicate review":
				input.Reviews = append(input.Reviews, input.Reviews[0])
			case "duplicate link":
				input.ReviewObservations = append(input.ReviewObservations, input.ReviewObservations[0])
			}
			if _, err := EncodePublicationReceipt(input); err == nil {
				t.Fatal("invalid review evidence was accepted")
			}
		})
	}
}

func TestPublicationReceiptBoundsReviewDecodeAllocations(t *testing.T) {
	for _, field := range []string{"reviews", "review_observations"} {
		t.Run(field, func(t *testing.T) {
			encoded, err := EncodePublicationReceipt(publicationReceiptFixture(t))
			if err != nil {
				t.Fatal(err)
			}
			var document map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &document); err != nil {
				t.Fatal(err)
			}
			document[field] = json.RawMessage("[" + strings.Repeat("{},", 100000) + "{}]")
			data, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			_, decodeErr := DecodePublicationReceipt(data)
			runtime.ReadMemStats(&after)
			if decodeErr == nil {
				t.Fatal("oversized review array was accepted")
			}
			const allocationBudget = 16 << 20
			if allocated := after.TotalAlloc - before.TotalAlloc; allocated > allocationBudget {
				t.Fatalf("review count refusal allocated %d bytes; budget %d", allocated, allocationBudget)
			}
		})
	}
}
