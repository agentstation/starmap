package sourceevidence

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/provenance"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestNormalizedEvidenceRoundTripReplaysCandidateAndProvenanceWithoutSecrets(t *testing.T) {
	const secret = "super-secret-provider-token"
	t.Setenv("STARMAP_EVIDENCE_SECRET", secret)
	builder := catalogs.NewEmpty()
	provider := catalogs.Provider{
		ID: "provider-a", Name: "Provider A",
		Credentials: map[catalogs.ProviderCredentialID]catalogs.ProviderCredential{"api_key": {Env: catalogs.ProviderEnvironmentNames{"STARMAP_EVIDENCE_SECRET"}}},
		Models:      map[string]*catalogs.Model{"model-a": {ID: "model-a", Name: "Model A"}},
	}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	builder.SetProvenance(provenance.Map{
		"models.provider-a.model-a.Name": {{
			Source: sources.ProvidersID, Field: "Name", Value: "Model A",
			Timestamp: time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC),
		}},
	})
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	observation, err := sources.NewObservation(sources.ProvidersID, catalog, sources.ObservationMetadata{
		ObservedAt: time.Date(2026, time.July, 10, 12, 1, 0, 0, time.UTC),
		Revision: sources.Revision{
			Kind: sources.RevisionKindGitCommit, Value: strings.Repeat("a", 40),
			InputName: "bun.lock", InputChecksum: "sha256:" + strings.Repeat("b", 64),
		},
		Completeness: sources.ObservationCompletenessPartial,
		Status:       sources.ObservationStatusDegraded,
		Records:      sources.ObservationRecordCounts{Accepted: 1, Rejected: 1},
		Issues: []sources.ObservationIssue{{
			Scope: sources.ObservationIssueScopeProvider, Code: sources.ObservationIssueCodeFetchFailed,
			Subject: "provider-b", Message: "request failed with " + secret,
		}},
		Acquisitions: []catalogmeta.AcquisitionProvenance{{
			ProviderID: "provider-a", SourceID: "models", AuthMethod: "api_key",
			Scope: catalogmeta.ObservationScopeGlobalPublic, Topology: catalogmeta.AcquisitionTopologySingleEndpoint,
		}},
	})
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}

	record, err := Capture(observation)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	persisted, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if bytes.Contains(persisted, []byte(secret)) {
		t.Fatal("normalized long-term evidence contains a credential or diagnostic secret")
	}
	var restored NormalizedRecord
	if err := json.Unmarshal(persisted, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	replayed, err := Replay(restored)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if replayed.ID != observation.ID || replayed.EvidenceChecksum != observation.EvidenceChecksum {
		t.Fatalf("replayed identity = (%q, %q), want (%q, %q)", replayed.ID, replayed.EvidenceChecksum, observation.ID, observation.EvidenceChecksum)
	}
	if replayed.Records != observation.Records || record.Records != observation.Records {
		t.Fatalf("record counts changed across evidence replay: record=%#v replay=%#v want=%#v", record.Records, replayed.Records, observation.Records)
	}
	replayedPayload, err := catalogs.EncodeCatalogPayload(replayed.Catalog)
	if err != nil {
		t.Fatalf("Encode replay: %v", err)
	}
	if !bytes.Equal(replayedPayload, record.Payload) {
		t.Fatal("replay did not reproduce candidate catalog and provenance bytes")
	}
}

func TestNormalizedEvidenceRejectsCredentialScopedObservationBeforeEncoding(t *testing.T) {
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	observation, err := sources.NewObservation(sources.ProvidersID, catalog, sources.ObservationMetadata{
		ObservedAt:   time.Date(2026, time.July, 14, 6, 45, 0, 0, time.UTC),
		Revision:     sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
		Scope: catalogmeta.ObservationScopeCredentialScoped, Kind: catalogmeta.SourceKindDirectInventory,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Capture(observation); err == nil {
		t.Fatal("Capture accepted credential-scoped publication evidence")
	}
	record := NormalizedRecord{Version: normalizedRecordVersion, Metrics: catalogmeta.ObservationMetrics{Scope: catalogmeta.ObservationScopeCredentialScoped}}
	if _, err := Replay(record); err == nil {
		t.Fatal("Replay accepted credential-scoped publication evidence")
	}
}

func TestRawEvidenceIsShortLivedEncryptedAndAccessControlledByPolicy(t *testing.T) {
	policy := DefaultPolicy()
	if err := policy.Validate(); err != nil {
		t.Fatalf("Validate policy: %v", err)
	}
	if policy.RawAccess != RawAccessOwnerOnly || policy.RawRetention > 7*24*time.Hour || !policy.RequireEncryption {
		t.Fatalf("unsafe default policy: %#v", policy)
	}
	key := bytes.Repeat([]byte{0x2a}, 32)
	record := RawRecord{
		SourceID:   catalogmeta.SourceID("provider-a"),
		ObservedAt: time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC),
		MediaType:  "application/json",
		Payload:    []byte(`{"models":["model-a"],"raw_secret":"short-lived"}`),
	}
	sealed, err := SealRaw(key, record, record.ObservedAt.Add(policy.RawRetention))
	if err != nil {
		t.Fatalf("SealRaw: %v", err)
	}
	encoded, err := json.Marshal(sealed)
	if err != nil {
		t.Fatalf("Marshal sealed: %v", err)
	}
	if strings.Contains(string(encoded), "model-a") || strings.Contains(string(encoded), "short-lived") {
		t.Fatal("sealed raw evidence exposes plaintext")
	}
	opened, err := OpenRaw(key, sealed, record.ObservedAt)
	if err != nil {
		t.Fatalf("OpenRaw: %v", err)
	}
	if !bytes.Equal(opened.Payload, record.Payload) {
		t.Fatalf("opened payload = %q, want %q", opened.Payload, record.Payload)
	}
	wrongKey := bytes.Repeat([]byte{0x3b}, 32)
	if _, err := OpenRaw(wrongKey, sealed, record.ObservedAt); err == nil {
		t.Fatal("wrong key opened raw evidence")
	}
	if _, err := OpenRaw(key, sealed, sealed.ExpiresAt.Add(time.Nanosecond)); err == nil {
		t.Fatal("expired raw evidence remained accessible")
	}
}
