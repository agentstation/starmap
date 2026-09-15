package publication

import (
	"bytes"
	"context"
	stderrors "errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/sources"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

func TestPreparePublicationBuildsVerifiedArtifactAndReceipt(t *testing.T) {
	profile, run := admissionFixture(t)
	empty, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	baseline := publicationBaselineFixture(t, empty)
	state, err := NewState(baseline, "test-publisher")
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := preparePublication(t.Context(), state, profile, run, "run-one")
	if err != nil {
		t.Fatal(err)
	}
	generation, err := artifact.Open(prepared.Bundle.Data, prepared.Bundle.Attestation)
	if err != nil {
		t.Fatal(err)
	}
	semantic, err := generation.SemanticChecksum()
	if err != nil {
		t.Fatal(err)
	}
	expected := artifact.PublicationArtifact{GenerationID: generation.Manifest.GenerationID, CatalogChecksum: semantic, PayloadChecksum: generation.Manifest.Payload.Checksum, ArchiveChecksum: prepared.Bundle.Checksum}
	receipt, err := artifact.VerifyPublicationReceipt(prepared.Receipt.Data, prepared.Receipt.Checksum, expected)
	if err != nil {
		t.Fatal(err)
	}
	if !receipt.FreshAcquisition || receipt.Sources[0].Observation.ObservationID != run.Attempts[0].Observation.ID {
		t.Fatal("source evidence did not reach the artifact receipt")
	}
	again, err := preparePublication(t.Context(), state, profile, run, "run-one")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(prepared.Bundle.Data, again.Bundle.Data) || !bytes.Equal(prepared.Receipt.Data, again.Receipt.Data) {
		t.Fatal("same run changed immutable output")
	}
}

func publicationBaselineFixture(t *testing.T, catalog *catalogs.Catalog) catalogs.Generation {
	t.Helper()
	generation, _, _ := receiptBindingFixture(t, catalog)
	generation.Manifest.SchemaVersion = catalogs.CatalogPayloadSchemaVersion(catalog)
	generation.Manifest.ConsumerCompatibility = catalogs.ConsumerCompatibility{MinSchemaVersion: generation.Manifest.SchemaVersion, MaxSchemaVersion: generation.Manifest.SchemaVersion}
	return generation
}

func TestPreparePublicationUsesPipelineAndRetainsFailedAccount(t *testing.T) {
	baseline, profile := producerPipelineFixture(t)
	state, err := NewState(publicationBaselineFixture(t, baseline), "fixture-publisher")
	if err != nil {
		t.Fatal(err)
	}
	var phase, requests atomic.Int32
	factory := func(*catalogs.Provider) (sources.ProviderClient, error) {
		return producerPipelineClient{phase: &phase, requests: &requests}, nil
	}
	resolver := sources.ProviderCredentialResolverFunc(func(_ context.Context, provider *catalogs.Provider) (sources.ProviderCredentialMaterial, error) {
		selected := provider.Credentials.CatalogAcquisition.Alternatives[0]
		for _, declared := range provider.Credentials.Profiles {
			if declared.ID == selected {
				return sources.NewProviderCredentialMaterial(declared, map[catalogs.ProviderCredentialFieldID]string{"api-key": "fixture"}, sources.ProviderCredentialMetadata{}), nil
			}
		}
		t.Fatal("missing fixture credential profile")
		return sources.ProviderCredentialMaterial{}, nil
	})
	producer, err := NewProducer(profile, factory, resolver)
	if err != nil {
		t.Fatal(err)
	}
	options := []pkgsync.Option{pkgsync.WithCatalogPath(t.TempDir())}
	first, err := producer.Prepare(t.Context(), state, "pipeline-one", options...)
	if err != nil {
		t.Fatal(err)
	}
	phase.Store(1)
	second, err := producer.Prepare(t.Context(), first.Next, "pipeline-two", options...)
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 4 || second.Decision.Scopes[1].EvidenceKind != RetainedEvidence {
		t.Fatalf("requests=%d decision=%+v", requests.Load(), second.Decision)
	}
	old, err := artifact.DecodePublicationReceipt(first.Receipt.Data)
	if err != nil {
		t.Fatal(err)
	}
	next, err := artifact.DecodePublicationReceipt(second.Receipt.Data)
	if err != nil {
		t.Fatal(err)
	}
	if old.Sources[1].Observation.ObservationID != next.Sources[1].Observation.ObservationID || !old.Sources[1].Observation.ObservedAt.Equal(next.Sources[1].Observation.ObservedAt) {
		t.Fatal("failed account received fresh evidence")
	}
	result, err := catalogs.DecodeCatalogGeneration(second.Next.Generation())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := result.Offering("provider", "two"); err != nil {
		t.Fatal("failed account lost its catalog entry", err)
	}
	if state.Generation().Manifest.GenerationID != "publication-receipt-generation" {
		t.Fatal("preparation advanced accepted state")
	}
	for i := range second.Bundle.Data {
		second.Bundle.Data[i] = 0
	}
	if _, err := artifact.Open(second.Next.bundle.Data, second.Next.bundle.Attestation); err != nil {
		t.Fatal("caller changed retained artifact", err)
	}
}

func TestPreparePublicationRejectsMissingAndCanceledEvidenceWithoutStateChange(t *testing.T) {
	profile, run := admissionFixture(t)
	empty, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	state, err := NewState(publicationBaselineFixture(t, empty), "fixture-publisher")
	if err != nil {
		t.Fatal(err)
	}
	original := state.Generation()
	run.Attempts[0].Outcome, run.Attempts[0].Observation = Failed, nil
	rejected, err := preparePublication(t.Context(), state, profile, run, "failed")
	if err == nil || rejected.Next != nil || len(rejected.Bundle.Data) != 0 || len(rejected.Receipt.Data) != 0 {
		t.Fatal("rejected run produced publishable output")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	canceled, err := preparePublication(ctx, state, profile, run, "canceled")
	if !stderrors.Is(err, context.Canceled) || canceled.Next != nil {
		t.Fatal("canceled run produced a successor", err)
	}
	if !bytes.Equal(original.Payload, state.Generation().Payload) || len(state.history) != 0 {
		t.Fatal("rejection changed accepted state")
	}
}

func TestPreparePublicationPreservesDisabledScopeUntilExplicitRemoval(t *testing.T) {
	profile, run := admissionFixture(t)
	empty, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	state, err := NewState(publicationBaselineFixture(t, empty), "fixture-publisher")
	if err != nil {
		t.Fatal(err)
	}
	first, err := preparePublication(t.Context(), state, profile, run, "one")
	if err != nil {
		t.Fatal(err)
	}
	profile.Scopes[0].Enabled = false
	run.StartedAt = run.StartedAt.Add(time.Minute)
	run.CompletedAt = run.StartedAt
	run.Attempts = []Attempt{{Scope: profile.Scopes[0].Scope, Outcome: Disabled}}
	run.Retained = nil
	kept, err := preparePublication(t.Context(), first.Next, profile, run, "kept")
	if err != nil {
		t.Fatal(err)
	}
	if !kept.ReusedArtifact || len(kept.Next.history) != 1 || kept.Decision.FreshAcquisition {
		t.Fatal("disabled scope lost prior facts or claimed acquisition")
	}
	profile.Scopes[0].DisabledAction = Remove
	removed, err := preparePublication(t.Context(), kept.Next, profile, run, "removed")
	if err != nil {
		t.Fatal(err)
	}
	if len(removed.Next.history) != 0 || removed.ReusedArtifact {
		t.Fatal("explicit removal kept local scope evidence")
	}
	catalog, err := catalogs.DecodeCatalogGeneration(removed.Next.Generation())
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.MembershipScopes()) != 0 {
		t.Fatal("removed local scope survived replay")
	}
	if len(first.Next.history) != 1 {
		t.Fatal("successor mutated its predecessor")
	}
}
