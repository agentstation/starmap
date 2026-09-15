package artifact

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/evidence"
)

func TestPublicationReceiptRefreshDoesNotChangeArtifact(t *testing.T) {
	generation := artifactFixtureGeneration(t)
	bundle, err := Build(generation)
	if err != nil {
		t.Fatal(err)
	}
	semantic, err := generation.SemanticChecksum()
	if err != nil {
		t.Fatal(err)
	}
	binding := PublicationArtifact{GenerationID: generation.Manifest.GenerationID, CatalogChecksum: semantic, PayloadChecksum: generation.Manifest.Payload.Checksum, ArchiveChecksum: bundle.Checksum}
	observed := generation.Manifest.SourceObservations[0]
	observed.Source = evidence.ModelsDevHTTPID
	first := PublicationReceipt{SchemaVersion: PublicationReceiptSchemaVersion, RunID: "run-1", StartedAt: observed.ObservedAt, CompletedAt: observed.ObservedAt, PolicyVersion: "policy-1", Artifact: binding, FreshAcquisition: true, Sources: []PublicationSourceReceipt{{Policy: PublicationScopePolicy{Source: observed.Source, Required: true, Enabled: true, MaxRetainedAge: 5 * time.Hour, DisabledAction: "preserve"}, Attempt: "succeeded", EvidenceKind: "fresh", Observation: &observed}}}
	firstData, err := EncodePublicationReceipt(first)
	if err != nil {
		t.Fatal(err)
	}
	second := first
	second.RunID = "run-2"
	second.StartedAt = first.StartedAt.Add(4 * time.Hour)
	second.CompletedAt = second.StartedAt
	second.FreshAcquisition = false
	second.Sources = append([]PublicationSourceReceipt(nil), first.Sources...)
	second.Sources[0].Attempt = "missing_credentials"
	second.Sources[0].EvidenceKind = "retained"
	secondData, err := EncodePublicationReceipt(second)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(firstData, secondData) {
		t.Fatal("new run replaced an old receipt")
	}
	for _, data := range [][]byte{firstData, secondData} {
		got, err := VerifyPublicationReceipt(data, checksum(data), binding)
		if err != nil {
			t.Fatal(err)
		}
		if got.Artifact != binding || got.Sources[0].Observation.ObservedAt != observed.ObservedAt {
			t.Fatal("receipt changed artifact or observation identity")
		}
	}
	reopened, err := Open(bundle.Data, bundle.Attestation)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Manifest.GenerationID != generation.Manifest.GenerationID || !bytes.Equal(reopened.Payload, generation.Payload) {
		t.Fatal("run confirmation changed immutable artifact bytes")
	}
}

func TestPublicationReceiptRejectsInvalidAdmissionClaims(t *testing.T) {
	tests := []struct {
		name   string
		change func(*PublicationReceipt)
	}{
		{"schema", func(r *PublicationReceipt) { r.SchemaVersion++ }},
		{"run id", func(r *PublicationReceipt) { r.RunID = "" }},
		{"unsafe run id", func(r *PublicationReceipt) { r.RunID = "run\nsecret" }},
		{"policy version", func(r *PublicationReceipt) { r.PolicyVersion = "" }},
		{"reversed times", func(r *PublicationReceipt) { r.CompletedAt = r.StartedAt.Add(-time.Second) }},
		{"non UTC time", func(r *PublicationReceipt) { r.StartedAt = r.StartedAt.In(time.FixedZone("offset", 3600)) }},
		{"generation id", func(r *PublicationReceipt) { r.Artifact.GenerationID = "" }},
		{"artifact checksum", func(r *PublicationReceipt) {
			r.Artifact.ArchiveChecksum = strings.TrimPrefix(r.Artifact.ArchiveChecksum, ChecksumPrefix)
		}},
		{"no source", func(r *PublicationReceipt) { r.Sources = nil }},
		{"too many sources", func(r *PublicationReceipt) { r.Sources = make([]PublicationSourceReceipt, maxPublicationScopes+1) }},
		{"duplicate source", func(r *PublicationReceipt) { r.Sources = append(r.Sources, r.Sources[0]) }},
		{"wrong source", func(r *PublicationReceipt) { r.Sources[0].Policy.Source = evidence.LocalCatalogID }},
		{"unknown source", func(r *PublicationReceipt) { r.Sources[0].Policy.Source = "unknown" }},
		{"required becomes optional", func(r *PublicationReceipt) { r.Sources[0].Policy.AllowMissing = true }},
		{"negative retained age", func(r *PublicationReceipt) { r.Sources[0].Policy.MaxRetainedAge = -1 }},
		{"implicit removal policy", func(r *PublicationReceipt) { r.Sources[0].Policy.DisabledAction = "" }},
		{"binding on metadata source", func(r *PublicationReceipt) { r.Sources[0].Policy.Binding = &PublicationBinding{} }},
		{"unbound provider source", func(r *PublicationReceipt) { r.Sources[0].Policy.Source = evidence.ProvidersID }},
		{"unknown attempt", func(r *PublicationReceipt) { r.Sources[0].Attempt = "unknown" }},
		{"disabled enabled scope", func(r *PublicationReceipt) { r.Sources[0].Attempt = "disabled" }},
		{"active disabled scope", func(r *PublicationReceipt) { r.Sources[0].Policy.Enabled = false }},
		{"missing successful evidence", func(r *PublicationReceipt) { r.Sources[0].Observation = nil }},
		{"false evidence kind", func(r *PublicationReceipt) { r.Sources[0].EvidenceKind = "none" }},
		{"unknown evidence kind", func(r *PublicationReceipt) { r.Sources[0].EvidenceKind = "unknown" }},
		{"partial evidence", func(r *PublicationReceipt) {
			r.Sources[0].Observation.Completeness = evidence.ObservationCompletenessPartial
			r.Sources[0].Observation.Status = evidence.ObservationStatusDegraded
		}},
		{"freshness overclaim", func(r *PublicationReceipt) { r.FreshAcquisition = false }},
		{"fresh observation before start", func(r *PublicationReceipt) {
			r.StartedAt = r.StartedAt.Add(time.Nanosecond)
			r.CompletedAt = r.StartedAt
		}},
		{"fresh observation after finish", func(r *PublicationReceipt) { r.StartedAt = r.StartedAt.Add(-time.Hour); r.CompletedAt = r.StartedAt }},
		{"retained evidence after start", func(r *PublicationReceipt) {
			r.Sources[0].Attempt = "failed"
			r.Sources[0].EvidenceKind = "retained"
			r.FreshAcquisition = false
			r.StartedAt = r.StartedAt.Add(-time.Hour)
		}},
		{"retained with zero age", func(r *PublicationReceipt) {
			r.Sources[0].Attempt = "failed"
			r.Sources[0].EvidenceKind = "retained"
			r.FreshAcquisition = false
			r.Sources[0].Policy.MaxRetainedAge = 0
		}},
		{"retained past age", func(r *PublicationReceipt) {
			r.Sources[0].Attempt = "failed"
			r.Sources[0].EvidenceKind = "retained"
			r.FreshAcquisition = false
			r.CompletedAt = r.CompletedAt.Add(time.Hour + time.Nanosecond)
		}},
		{"missing required evidence", func(r *PublicationReceipt) {
			r.Sources[0].Attempt = "missing_credentials"
			r.Sources[0].EvidenceKind = "none"
			r.Sources[0].Observation = nil
			r.FreshAcquisition = false
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receipt := publicationReceiptFixture(t)
			tt.change(&receipt)
			if err := receipt.Validate(); err == nil {
				t.Fatal("invalid admission claim passed validation")
			}
			if data, err := EncodePublicationReceipt(receipt); err == nil || len(data) != 0 {
				t.Fatalf("invalid claim produced receipt bytes: %d, %v", len(data), err)
			}
		})
	}
}

func TestPublicationReceiptDistinguishesFallbackAndOptionalFailure(t *testing.T) {
	for _, state := range []string{"retained", "embedded", "optional", "disabled-preserve", "disabled-remove"} {
		t.Run(state, func(t *testing.T) {
			receipt := publicationReceiptFixture(t)
			receipt.FreshAcquisition = false
			source := &receipt.Sources[0]
			switch state {
			case "retained":
				source.Attempt = "partial"
				source.EvidenceKind = "retained"
				receipt.StartedAt = receipt.StartedAt.Add(time.Hour)
				receipt.CompletedAt = receipt.StartedAt
			case "embedded":
				source.Policy.Source = evidence.EmbeddedCatalogID
				source.Observation.Source = evidence.EmbeddedCatalogID
			case "optional":
				source.Policy.Required = false
				source.Policy.AllowMissing = true
				source.Attempt = "missing_credentials"
				source.EvidenceKind = "none"
				source.Observation = nil
			default:
				source.Policy.Enabled = false
				source.Policy.DisabledAction = strings.TrimPrefix(state, "disabled-")
				source.Attempt = "disabled"
				source.EvidenceKind = "none"
				source.Observation = nil
			}
			data, err := EncodePublicationReceipt(receipt)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := VerifyPublicationReceipt(data, checksum(data), receipt.Artifact)
			if err != nil {
				t.Fatal(err)
			}
			if decoded.FreshAcquisition {
				t.Fatal("fallback claimed fresh acquisition")
			}
		})
	}
}

func TestPublicationReceiptBindsProviderScopeAndArtifact(t *testing.T) {
	receipt := publicationReceiptFixture(t)
	receipt.Sources[0].Policy.Source = evidence.ProvidersID
	receipt.Sources[0].Observation.Source = evidence.ProvidersID
	receipt.Sources[0].Policy.Binding = &PublicationBinding{ID: "scope-a", Revision: "1", ProviderID: "provider", Checksum: testCatalogDigest}
	data, err := EncodePublicationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	receipt.Sources[0].Policy.Binding.Revision = "2"
	changed, err := EncodePublicationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyPublicationReceipt(changed, checksum(data), receipt.Artifact); err == nil {
		t.Fatal("changed binding accepted under original receipt digest")
	}
	for _, field := range []string{"generation", "catalog", "payload", "archive"} {
		t.Run(field, func(t *testing.T) {
			binding := receipt.Artifact
			switch field {
			case "generation":
				binding.GenerationID = "different"
			case "catalog":
				binding.CatalogChecksum = testCatalogDigest2
			case "payload":
				binding.PayloadChecksum = testCatalogDigest2
			case "archive":
				binding.ArchiveChecksum = testCatalogDigest2
			}
			if _, err := VerifyPublicationReceipt(data, checksum(data), binding); err == nil {
				t.Fatal("receipt accepted against another artifact")
			}
		})
	}
	receipt.Sources[0].Policy.Binding.Checksum = "invalid"
	if err := receipt.Validate(); err == nil {
		t.Fatal("unbound provider scope accepted")
	}
}

func TestPublicationReceiptRejectsAmbiguousOrOversizedEncoding(t *testing.T) {
	receipt := publicationReceiptFixture(t)
	data, err := EncodePublicationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"duplicate field":     bytes.Replace(data, []byte(`"schema_version": 1,`), []byte(`"schema_version": 1, "schema_version": 1,`), 1),
		"omitted false field": bytes.Replace(data, []byte("        \"allow_missing\": false,\n"), nil, 1),
		"unknown field":       bytes.Replace(data, []byte(`"run_id":`), []byte(`"extra": false, "run_id":`), 1),
		"case variant":        bytes.Replace(data, []byte(`"run_id":`), []byte(`"Run_ID":`), 1),
		"trailing value":      append(bytes.Clone(data), []byte("{}")...),
		"oversized":           bytes.Repeat([]byte(" "), maxPublicationReceiptBytes+1),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(raw, data) {
				t.Fatal("malformed fixture did not change encoding")
			}
			if _, err := DecodePublicationReceipt(raw); err == nil {
				t.Fatal("invalid encoding accepted")
			}
		})
	}
}

func publicationReceiptFixture(t *testing.T) PublicationReceipt {
	t.Helper()
	generation := artifactFixtureGeneration(t)
	bundle, err := Build(generation)
	if err != nil {
		t.Fatal(err)
	}
	semantic, err := generation.SemanticChecksum()
	if err != nil {
		t.Fatal(err)
	}
	observation := generation.Manifest.SourceObservations[0]
	observation.Source = evidence.ModelsDevHTTPID
	// The manifest fixture supplies complete receipt metadata for this codec test.
	observation.Completeness = evidence.ObservationCompletenessComplete
	observation.Status = evidence.ObservationStatusSucceeded
	return PublicationReceipt{SchemaVersion: PublicationReceiptSchemaVersion, RunID: "test-run", StartedAt: observation.ObservedAt, CompletedAt: observation.ObservedAt, PolicyVersion: "policy-1", Artifact: PublicationArtifact{GenerationID: generation.Manifest.GenerationID, CatalogChecksum: semantic, PayloadChecksum: generation.Manifest.Payload.Checksum, ArchiveChecksum: bundle.Checksum}, FreshAcquisition: true, Sources: []PublicationSourceReceipt{{Policy: PublicationScopePolicy{Source: observation.Source, Required: true, Enabled: true, MaxRetainedAge: time.Hour, DisabledAction: "preserve"}, Attempt: "succeeded", EvidenceKind: "fresh", Observation: &observation}}}
}

func TestPublicationReceiptBoundsSourceDecodeAllocations(t *testing.T) {
	receipt := publicationReceiptFixture(t)
	encoded, err := EncodePublicationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	document["sources"] = json.RawMessage("[" + strings.Repeat("{},", 100000) + "{}]")
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) >= maxPublicationReceiptBytes {
		t.Fatal("fixture must fit the byte limit")
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, decodeErr := DecodePublicationReceipt(data)
	runtime.ReadMemStats(&after)
	if decodeErr == nil {
		t.Fatal("receipt exceeds the source count limit")
	}
	// A small hostile array must not allocate all of its typed source records.
	const allocationBudget = 16 << 20
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > allocationBudget {
		t.Fatalf("source count refusal allocated %d bytes; budget %d", allocated, allocationBudget)
	} else {
		t.Logf("source count refusal allocated %d bytes; budget %d", allocated, allocationBudget)
	}
}

func TestPublicationReceiptSourceArrayBoundaries(t *testing.T) {
	receipt := publicationReceiptFixture(t)
	receipt.FreshAcquisition = false
	receipt.Sources = make([]PublicationSourceReceipt, maxPublicationScopes)
	for i := range receipt.Sources {
		receipt.Sources[i] = PublicationSourceReceipt{
			Policy: PublicationScopePolicy{
				Source: evidence.ProvidersID, Enabled: true, AllowMissing: true, DisabledAction: "preserve",
				Binding: &PublicationBinding{ID: strconv.Itoa(i), Revision: "1", ProviderID: "openai", Checksum: receipt.Artifact.CatalogChecksum},
			},
			Attempt: "not_attempted", EvidenceKind: "none",
		}
	}
	data, err := EncodePublicationReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodePublicationReceipt(data)
	if err != nil || len(decoded.Sources) != maxPublicationScopes {
		t.Fatalf("maximum source count did not round trip: %d, %v", len(decoded.Sources), err)
	}
	extra := receipt.Sources[0]
	binding := *extra.Policy.Binding
	binding.ID = "excess"
	extra.Policy.Binding = &binding
	receipt.Sources = append(receipt.Sources, extra)
	data, err = json.MarshalIndent(receipt, "", channelIndent)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if len(data) >= maxPublicationReceiptBytes {
		t.Fatal("source count boundary must fit the byte limit")
	}
	if _, err := DecodePublicationReceipt(data); err == nil {
		t.Fatal("decoder accepted one excess source")
	}
}
