package bootstrapmanifest

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/testcatalog"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/provenance"
)

func TestScheduledGenerationManifestChangesOnlyForCanonicalPayloadChange(t *testing.T) {
	builder := catalogs.NewEmpty()
	author := testAuthor()
	if err := builder.SetAuthor(*author); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	provider := testProvider()
	model := testModel()
	model.ModelRef = catalogs.AuthoredModelID(author.ID, model.ID)
	if err := builder.SetAuthorModel(author.ID, catalogs.Model{
		ID:      model.ID,
		Name:    model.Name,
		Authors: []catalogs.Author{*author},
	}); err != nil {
		t.Fatalf("SetAuthorModel: %v", err)
	}
	provider.Models = map[string]*catalogs.Model{model.ID: model}
	if err := builder.SetProvider(*provider); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	firstCatalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build first: %v", err)
	}
	firstTime := time.Date(2026, time.July, 10, 15, 0, 0, 0, time.UTC)
	first, firstReport, err := Derive(firstCatalog, nil, firstTime)
	if err != nil {
		t.Fatalf("Derive first: %v", err)
	}
	if !firstReport.Changed || first.GenerationID == "" {
		t.Fatalf("first report/manifest = %#v/%#v", firstReport, first)
	}
	unchanged, unchangedReport, err := Derive(firstCatalog, &first, firstTime.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("Derive unchanged: %v", err)
	}
	if unchangedReport.Changed || unchanged != first {
		t.Fatalf("unchanged report/manifest = %#v/%#v, want exact current", unchangedReport, unchanged)
	}
	provider.Models[model.ID].Name = "Changed canonical model name"
	if err := builder.SetProvider(*provider); err != nil {
		t.Fatalf("SetProvider changed: %v", err)
	}
	changedCatalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build changed: %v", err)
	}
	changed, changedReport, err := Derive(changedCatalog, &first, firstTime.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("Derive changed: %v", err)
	}
	if !changedReport.Changed || changed.GenerationID == first.GenerationID ||
		changed.Payload.Checksum == first.Payload.Checksum || changed.GeneratedAt != firstTime.Add(24*time.Hour) {
		t.Fatalf("changed report/manifest = %#v/%#v", changedReport, changed)
	}
}

func TestScheduledGenerationManifestRebindsChangedEvidencePayload(t *testing.T) {
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(*testProvider()); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	firstTime := time.Date(2026, time.July, 29, 20, 0, 0, 0, time.UTC)
	setEvidence := func(observedAt time.Time, id string) {
		builder.SetProvenance(provenance.Map{
			"providers.test-provider.Name": {{
				Source: catalogmeta.ProvidersID, Field: "Name", Value: "Test Provider",
				Timestamp: observedAt, ObservedAt: observedAt, ObservationID: id,
				EvidenceChecksum: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			}},
		})
	}
	setEvidence(firstTime, "observation:first")
	firstCatalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build first: %v", err)
	}
	first, _, err := Derive(firstCatalog, nil, firstTime)
	if err != nil {
		t.Fatalf("Derive first: %v", err)
	}

	setEvidence(firstTime.Add(time.Hour), "observation:second")
	secondCatalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build second: %v", err)
	}
	second, report, err := Derive(secondCatalog, &first, firstTime.Add(time.Hour))
	if err != nil {
		t.Fatalf("Derive second: %v", err)
	}
	if !report.Changed {
		t.Fatalf("evidence-only payload change was not rebound: %#v / %#v", report, second)
	}
	if second.SemanticChecksum != first.SemanticChecksum {
		t.Fatalf("semantic checksum changed for evidence-only update: %q != %q", second.SemanticChecksum, first.SemanticChecksum)
	}
	if second.Payload == first.Payload || second.GenerationID == first.GenerationID {
		t.Fatalf("exact evidence payload retained stale identity: %#v / %#v", report, second)
	}
	secondPayload, err := catalogs.EncodeCatalogPayload(secondCatalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload second: %v", err)
	}
	if catalogs.DescribeCatalogPayload(secondPayload) == first.Payload {
		t.Fatal("fixture did not change the exact evidence payload")
	}
}

func TestDeriveCommittedPreservesExactGenerationIdentity(t *testing.T) {
	catalog, generation := committedFixture(t, "committed-generation")

	manifest, report, err := DeriveCommitted(catalog, generation, nil)
	if err != nil {
		t.Fatalf("DeriveCommitted: %v", err)
	}
	if manifest.GenerationID != generation.Manifest.GenerationID ||
		manifest.GeneratedAt != generation.Manifest.GeneratedAt ||
		manifest.Payload != generation.Manifest.Payload {
		t.Fatalf("manifest = %#v, generation = %#v", manifest, generation.Manifest)
	}
	if !report.Changed || report.GenerationID != generation.Manifest.GenerationID {
		t.Fatalf("report = %#v", report)
	}

	unchanged, unchangedReport, err := DeriveCommitted(
		catalog,
		generation,
		&manifest,
	)
	if err != nil {
		t.Fatalf("DeriveCommitted unchanged: %v", err)
	}
	if unchanged != manifest || unchangedReport.Changed {
		t.Fatalf("unchanged = %#v / %#v", unchanged, unchangedReport)
	}
}

func TestDeriveCommittedRejectsCatalogDifferentFromCommittedPayload(t *testing.T) {
	catalog, generation := committedFixture(t, "committed-generation")
	differentBuilder := catalogs.NewEmpty()
	if err := differentBuilder.SetAuthor(*testAuthor()); err != nil {
		t.Fatalf("SetAuthor: %v", err)
	}
	different, err := differentBuilder.Build()
	if err != nil {
		t.Fatalf("Build different catalog: %v", err)
	}
	if _, _, err := DeriveCommitted(different, generation, nil); err == nil {
		t.Fatal("DeriveCommitted accepted catalog bytes different from the commit")
	}
	if _, _, err := DeriveCommitted(catalog, catalogstore.Generation{}, nil); err == nil {
		t.Fatal("DeriveCommitted accepted an invalid committed generation")
	}
}

func committedFixture(
	t *testing.T,
	id string,
) (*catalogs.Catalog, catalogstore.Generation) {
	t.Helper()
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatalf("Build empty catalog: %v", err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	generatedAt := time.Date(2026, time.July, 29, 20, 0, 0, 0, time.UTC)
	descriptor := catalogs.DescribeCatalogPayload(payload)
	generation := catalogstore.Generation{
		Manifest: catalogs.GenerationManifest{
			ManifestVersion: catalogs.CurrentGenerationManifestVersion,
			SchemaVersion:   catalogs.CurrentCatalogSchemaVersion,
			GenerationID:    id,
			GeneratedAt:     generatedAt,
			Payload:         descriptor,
			Validation: catalogs.GenerationValidationReport{
				ValidatorVersion: "test/v1",
				ValidatedAt:      generatedAt,
				Status:           catalogs.GenerationValidationPassed,
				Checks: []catalogs.GenerationValidationCheck{{
					Name: "catalog", Status: catalogs.GenerationValidationCheckPassed,
				}},
			},
			SyncRunID: "sync-" + id,
			SourceObservations: []catalogs.SourceObservationLink{
				{
					Source:        catalogmeta.ModelsDevGitID,
					ObservationID: "modelsdev-observation",
					ObservedAt:    generatedAt,
					Revision: catalogmeta.ObservationRevision{
						Kind:  catalogmeta.ObservationRevisionKindContentDigest,
						Value: descriptor.Checksum,
					},
					Completeness:     catalogmeta.ObservationCompletenessComplete,
					Status:           catalogmeta.ObservationStatusSucceeded,
					EvidenceChecksum: descriptor.Checksum,
				},
			},
			ReviewCandidates: []catalogmeta.ReviewCandidate{},
			Completeness:     catalogs.GenerationCompletenessComplete,
			ConsumerCompatibility: catalogs.ConsumerCompatibility{
				MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
				MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
			},
		},
		Payload: payload,
	}
	if err := generation.Validate(); err != nil {
		t.Fatalf("Validate generation: %v", err)
	}
	return catalog, generation
}

func testAuthor() *catalogs.Author {
	return &catalogs.Author{ID: "test-author", Name: "Test Author"}
}

func testProvider() *catalogs.Provider {
	return &catalogs.Provider{
		ID:   "test-provider",
		Name: "Test Provider",
		Credentials: testcatalog.APIKeyCredentials(
			"TEST_PROVIDER_API_KEY", "Authorization", catalogs.ProviderCredentialSchemeBearer,
		),
		Catalog: &catalogs.ProviderCatalog{},
	}
}

func testModel() *catalogs.Model {
	return &catalogs.Model{
		ID:          "test-model",
		Name:        "Test Model",
		Description: "A test model for unit tests",
	}
}
