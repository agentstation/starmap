package publication

import (
	"bytes"
	"context"
	stderrors "errors"
	"os"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestReceiptBindingUsesExactArtifactAndSourceEvidence(t *testing.T) {
	profile, run := admissionFixture(t)
	generation, bundle, semantic := receiptBindingFixture(t, run.Attempts[0].Observation.Catalog)
	record, err := BindReceipt(t.Context(), profile, run, "publication-run-one", semantic, bundle.Data, bundle.Attestation)
	if err != nil {
		t.Fatal(err)
	}
	expected := artifact.PublicationArtifact{GenerationID: generation.Manifest.GenerationID, CatalogChecksum: semantic, PayloadChecksum: generation.Manifest.Payload.Checksum, ArchiveChecksum: bundle.Checksum}
	receipt, err := artifact.VerifyPublicationReceipt(record.Data, record.Checksum, expected)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Sources[0].Observation.ObservationID != run.Attempts[0].Observation.ID || !receipt.FreshAcquisition {
		t.Fatal("receipt changed original source evidence")
	}
	if bytes.Contains(record.Data, []byte(profile.Scopes[0].Scope.Binding.AccountID)) || bytes.Contains(record.Data, []byte("credential_profile_id")) {
		t.Fatal("receipt exposed private binding selectors")
	}
	again, err := BindReceipt(t.Context(), profile, run, "publication-run-one", semantic, bundle.Data, bundle.Attestation)
	if err != nil || !bytes.Equal(record.Data, again.Data) || record.Checksum != again.Checksum {
		t.Fatalf("receipt retry changed bytes: %v", err)
	}
}

func TestReceiptBindingRejectsInvalidAdmissionAndArtifact(t *testing.T) {
	for _, name := range []string{"admission", "semantic", "archive", "statement", "run_identity"} {
		t.Run(name, func(t *testing.T) {
			profile, run := admissionFixture(t)
			_, bundle, semantic := receiptBindingFixture(t, run.Attempts[0].Observation.Catalog)
			runID := "publication-run"
			switch name {
			case "admission":
				run.Attempts[0].Outcome, run.Attempts[0].Observation = Failed, nil
			case "semantic":
				semantic = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			case "archive":
				bundle.Data[len(bundle.Data)-1] ^= 1
			case "statement":
				bundle.Attestation = []byte("{}")
			case "run_identity":
				runID = ""
			}
			result, err := BindReceipt(t.Context(), profile, run, runID, semantic, bundle.Data, bundle.Attestation)
			if err == nil || len(result.Data) != 0 || result.Checksum != "" {
				t.Fatalf("invalid receipt escaped: %+v, %v", result, err)
			}
		})
	}
}

func TestReceiptBindingReusesArtifactWithoutChangingSourceFreshness(t *testing.T) {
	profile, firstRun := admissionFixture(t)
	_, bundle, semantic := receiptBindingFixture(t, firstRun.Attempts[0].Observation.Catalog)
	archive, statement := bytes.Clone(bundle.Data), bytes.Clone(bundle.Attestation)
	first, err := BindReceipt(t.Context(), profile, firstRun, "run-one", semantic, archive, statement)
	if err != nil {
		t.Fatal(err)
	}
	freshRun := firstRun
	freshRun.StartedAt = firstRun.StartedAt.Add(time.Minute)
	freshRun.CompletedAt = freshRun.StartedAt
	fresh := admissionObservation(t, profile.Scopes[0].Scope, freshRun.StartedAt)
	freshRun.Attempts = []Attempt{{Scope: profile.Scopes[0].Scope, Outcome: Succeeded, Observation: &fresh}}
	second, err := BindReceipt(t.Context(), profile, freshRun, "run-two", semantic, archive, statement)
	if err != nil {
		t.Fatal(err)
	}
	retainedRun := Run{StartedAt: freshRun.StartedAt.Add(time.Minute), CompletedAt: freshRun.StartedAt.Add(time.Minute), Attempts: []Attempt{{Scope: profile.Scopes[0].Scope, Outcome: Failed}}, Retained: []sources.Observation{fresh}}
	third, err := BindReceipt(t.Context(), profile, retainedRun, "run-three", semantic, archive, statement)
	if err != nil {
		t.Fatal(err)
	}
	firstReceipt, err := artifact.DecodePublicationReceipt(first.Data)
	if err != nil {
		t.Fatal(err)
	}
	for i, record := range []ReceiptRecord{second, third} {
		receipt, err := artifact.VerifyPublicationReceipt(record.Data, record.Checksum, firstReceipt.Artifact)
		if err != nil {
			t.Fatal(err)
		}
		if receipt.Sources[0].Observation.ObservationID != fresh.ID || receipt.FreshAcquisition != (i == 0) {
			t.Fatalf("source freshness=%+v", receipt)
		}
		if record.Checksum == first.Checksum || second.Checksum == third.Checksum {
			t.Fatal("different runs reused a receipt")
		}
	}
	if !bytes.Equal(archive, bundle.Data) || !bytes.Equal(statement, bundle.Attestation) {
		t.Fatal("receipt binding changed immutable artifact bytes")
	}
}

func TestReceiptBindingCoversPrivateSelectorsWithoutPublishingThem(t *testing.T) {
	profile, run := admissionFixture(t)
	_, bundle, semantic := receiptBindingFixture(t, run.Attempts[0].Observation.Catalog)
	initial, err := BindReceipt(t.Context(), profile, run, "run-one", semantic, bundle.Data, bundle.Attestation)
	if err != nil {
		t.Fatal(err)
	}
	original, err := artifact.DecodePublicationReceipt(initial.Data)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"account", "project", "region", "surface", "credential_profile"} {
		t.Run(field, func(t *testing.T) {
			changed, changedRun := admissionFixture(t)
			binding := changed.Scopes[0].Scope.Binding
			value := "private-" + field
			switch field {
			case "account":
				binding.AccountID = value
			case "project":
				binding.ProjectID = value
			case "region":
				binding.Region = value
			case "surface":
				binding.APISurface = value
			case "credential_profile":
				binding.CredentialProfileID = catalogs.ProviderCredentialProfileID(value)
			}
			observation := admissionObservation(t, changed.Scopes[0].Scope, changedRun.StartedAt)
			changedRun.Attempts = []Attempt{{Scope: changed.Scopes[0].Scope, Outcome: Succeeded, Observation: &observation}}
			record, err := BindReceipt(t.Context(), changed, changedRun, "run-one", semantic, bundle.Data, bundle.Attestation)
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := artifact.DecodePublicationReceipt(record.Data)
			if err != nil {
				t.Fatal(err)
			}
			before, after := original.Sources[0].Policy.Binding, receipt.Sources[0].Policy.Binding
			if before.ID != after.ID || before.Revision != after.Revision || before.Checksum == after.Checksum {
				t.Fatal("binding digest omitted the changed selector")
			}
			if bytes.Contains(record.Data, []byte(value)) {
				t.Fatal("receipt published a private selector")
			}
		})
	}
}

func TestReceiptBindingRejectsCancellation(t *testing.T) {
	profile, run := admissionFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	record, err := BindReceipt(ctx, profile, run, "run-one", "", nil, nil)
	if !stderrors.Is(err, context.Canceled) || len(record.Data) != 0 {
		t.Fatalf("canceled receipt=%+v, %v", record, err)
	}
}

func receiptBindingFixture(t *testing.T, catalog *catalogs.Catalog) (catalogs.Generation, artifact.Bundle, string) {
	t.Helper()
	data, err := os.ReadFile("../../../pkg/catalogs/testdata/generation/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatal(err)
	}
	manifest.GenerationID = "publication-receipt-generation"
	manifest.Payload = catalogs.DescribeCatalogPayload(payload)
	generation := catalogs.Generation{Manifest: manifest, Payload: payload}
	bundle, err := artifact.Build(generation)
	if err != nil {
		t.Fatal(err)
	}
	semantic, err := generation.SemanticChecksum()
	if err != nil {
		t.Fatal(err)
	}
	return generation, bundle, semantic
}
