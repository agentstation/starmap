package runtime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestRetainedProviderReceiptSurvivesRestart(t *testing.T) {
	store, err := newLayerStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	layer := testProviderLayer(t, "provider", "model", "Model", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
	catalog, err := catalogs.DecodeSourceObservationPayload(layer.Payload)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.ProvidersID, catalog, sources.ObservationMetadata{
		ObservedAt:   layer.ObservedAt,
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessPartial,
		Status:       sources.ObservationStatusDegraded,
		Records:      sources.ObservationRecordCounts{Accepted: 1, Rejected: 1},
		Issues:       []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord, Subject: "provider/invalid", Message: "private diagnostic sentinel"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt := map[string]any{
		"link": observation.Link(), "records": observation.Records,
		"issues": []map[string]any{{"scope": observation.Issues[0].Scope, "code": observation.Issues[0].Code, "subject": observation.Issues[0].Subject}},
	}
	record := providerReceiptRecord(t, layer)
	record["Receipt"], err = json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.writeContext(t.Context(), store.providers, "provider.json", record); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.loadProviders()
	if err != nil {
		t.Fatal(err)
	}
	retained := providerReceiptRecord(t, loaded[providerEvidenceKey{providerID: "provider"}])
	var want, got any
	if err := json.Unmarshal(record["Receipt"], &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(retained["Receipt"], &got); err != nil {
		t.Fatalf("retained receipt is missing or invalid: %v", err)
	}
	wantBytes, _ := json.Marshal(want)
	gotBytes, _ := json.Marshal(got)
	if string(wantBytes) != string(gotBytes) {
		t.Fatal("restart changed the source receipt")
	}
}

func TestRetainedProviderRefusesMissingReceipt(t *testing.T) {
	store, err := newLayerStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	layer := testProviderLayer(t, "provider", "model", "Model", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
	record := providerReceiptRecord(t, layer)
	delete(record, "Receipt")
	if err := store.writeContext(t.Context(), store.providers, "provider.json", record); err != nil {
		t.Fatal(err)
	}
	if _, err := store.loadProviders(); err == nil {
		t.Fatal("missing source receipt was accepted")
	}
}

func providerReceiptRecord(t *testing.T, layer ProviderLayer) map[string]json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(layer)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &record); err != nil {
		t.Fatal(err)
	}
	return record
}

func partialProviderReceiptLayer(t *testing.T) ProviderLayer {
	t.Helper()
	base := testProviderLayer(t, "provider", "model", "Model", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
	catalog, err := catalogs.DecodeSourceObservationPayload(base.Payload)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.ProvidersID, catalog, sources.ObservationMetadata{
		ObservedAt: base.ObservedAt, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessPartial, Status: sources.ObservationStatusDegraded,
		Records: sources.ObservationRecordCounts{Accepted: 1, Rejected: 1},
		Issues:  []sources.ObservationIssue{{Scope: sources.ObservationIssueScopeRecord, Code: sources.ObservationIssueCodeInvalidRecord, Subject: "provider/rejected", Message: "private diagnostic sentinel"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	layer, err := NewProviderLayer("provider", observation)
	if err != nil {
		t.Fatal(err)
	}
	return layer
}

func TestProviderReceiptMetadataConflictPreservesRetainedState(t *testing.T) {
	r := providerOrderRuntime(t, true)
	complete := testProviderLayer(t, "provider", "model", "Model", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
	if err := r.retainProviders(t.Context(), []ProviderLayer{complete}); err != nil {
		t.Fatal(err)
	}
	partial := partialProviderReceiptLayer(t)
	if complete.Digest != partial.Digest {
		t.Fatal("fixture payloads differ")
	}
	if err := r.retainProviders(t.Context(), []ProviderLayer{partial}); err == nil {
		t.Fatal("conflicting completeness was treated as an identical observation")
	}
	loaded, err := r.store.loadProviders()
	if err != nil {
		t.Fatal(err)
	}
	if loaded[providerEvidenceKey{providerID: "provider"}].Receipt.Link.ObservationID != complete.Receipt.Link.ObservationID {
		t.Fatal("conflict changed retained receipt")
	}
}

func TestProviderReceiptRetentionOwnsIssues(t *testing.T) {
	r := providerOrderRuntime(t, true)
	layer := partialProviderReceiptLayer(t)
	if err := r.retainProviders(t.Context(), []ProviderLayer{layer}); err != nil {
		t.Fatal(err)
	}
	layer.Receipt.Issues[0].Subject = "caller-mutation"
	if r.layers.providers[providerEvidenceKey{providerID: "provider"}].Receipt.Issues[0].Subject != "provider/rejected" {
		t.Fatal("caller changed retained receipt issues")
	}
	if _, err := r.store.loadProviders(); err != nil {
		t.Fatal(err)
	}
}

func TestProviderReceiptReportsDegradedAcquisition(t *testing.T) {
	layer := partialProviderReceiptLayer(t)
	acquirer := &stubAcquirer{result: AcquisitionResult{
		Eligible: 1,
		Attempts: []sources.ProviderAttempt{testAttempt("provider", sources.ProviderOutcomeSucceeded, "")},
		Layers:   []ProviderLayer{layer},
	}}
	r := openTestRuntime(t, WithCatalogSource("embedded"), WithAcquirer(acquirer))
	report, err := r.Refresh(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if report.Acquisition.Health != HealthDegraded || report.Acquisition.Succeeded != 1 || !report.Acquisition.Published {
		t.Fatalf("partial acquisition report = %+v", report.Acquisition)
	}
}

func TestProviderReceiptOversizedBatchChangesNoEvidence(t *testing.T) {
	r := providerOrderRuntime(t, true)
	layer := testProviderLayer(t, "provider", "model", "Model", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
	catalog, err := catalogs.DecodeSourceObservationPayload(layer.Payload)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.ProvidersID, catalog, sources.ObservationMetadata{
		ObservedAt: layer.ObservedAt, Revision: sources.Revision{Kind: sources.RevisionKindETag, Value: strings.Repeat("x", maxLayerBytes)},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatal(err)
	}
	layer.Receipt, err = observation.Receipt()
	if err != nil {
		t.Fatal(err)
	}
	first := testProviderLayer(t, "another", "model", "Model", layer.ObservedAt)
	if err := r.retainProviders(t.Context(), []ProviderLayer{first, layer}); err == nil {
		t.Fatal("oversized retained record accepted")
	}
	if len(r.layers.providers) != 0 {
		t.Fatal("oversized batch changed memory")
	}
	loaded, err := r.store.loadProviders()
	if err != nil || len(loaded) != 0 {
		t.Fatalf("oversized batch changed durable evidence: %v", err)
	}
}
