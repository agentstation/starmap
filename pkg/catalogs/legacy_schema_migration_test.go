package catalogs

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLegacySchemaMigrationPreservesProviderOfferings(t *testing.T) {
	legacy := NewEmpty()
	for _, provider := range []Provider{
		{
			ID: "provider-a", Name: "Provider A",
			Models: map[string]*Model{"shared": legacyMigrationModel("shared", 1.0, "priority")},
		},
		{
			ID: "provider-b", Name: "Provider B",
			Models: map[string]*Model{"shared": legacyMigrationModel("shared", 2.0, "standard")},
		},
	} {
		if err := legacy.SetProvider(provider); err != nil {
			t.Fatalf("SetProvider: %v", err)
		}
	}

	migrated, err := MigrateLegacySchema(legacy)
	if err != nil {
		t.Fatalf("MigrateLegacySchema: %v", err)
	}
	if len(migrated.Definitions) != 1 {
		t.Fatalf("definitions = %d, want 1", len(migrated.Definitions))
	}
	if len(migrated.Offerings) != 2 {
		t.Fatalf("offerings = %d, want 2", len(migrated.Offerings))
	}
	for _, providerID := range []ProviderID{"provider-a", "provider-b"} {
		key := OfferingKey{ProviderID: providerID, ProviderModelID: "shared"}
		offering, found := migrated.Offerings[key]
		if !found {
			t.Fatalf("missing offering %#v", key)
		}
		if offering.DefinitionID != "shared" {
			t.Fatalf("offering definition = %q, want shared", offering.DefinitionID)
		}
	}
	if got := migrated.Offerings[OfferingKey{ProviderID: "provider-a", ProviderModelID: "shared"}].Pricing.Tokens.Input.Per1M; got != 1.0 {
		t.Fatalf("provider-a price = %v, want 1", got)
	}
	if got := string(migrated.Offerings[OfferingKey{ProviderID: "provider-b", ProviderModelID: "shared"}].Modes["fast"].Request.Body["service_tier"]); got != `"standard"` {
		t.Fatalf("provider-b mode body = %s", got)
	}
	assertMigrationChangesClassified(t, migrated.Report)
}

func TestLegacySchemaMigrationDoesNotTreatMarketplaceAuthorListAsJointAuthorship(t *testing.T) {
	legacy := NewEmpty()
	for _, author := range []Author{{ID: "author-a", Name: "Author A"}, {ID: "author-b", Name: "Author B"}} {
		if err := legacy.SetAuthor(author); err != nil {
			t.Fatalf("SetAuthor: %v", err)
		}
	}
	if err := legacy.SetProvider(Provider{
		ID: "marketplace", Name: "Marketplace",
		Catalog: &ProviderCatalog{Authors: []AuthorID{"author-a", "author-b"}},
		Models:  map[string]*Model{"native-model": {ID: "native-model", Name: "Native Model"}},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	migrated, err := MigrateLegacySchema(legacy)
	if err != nil {
		t.Fatalf("MigrateLegacySchema: %v", err)
	}
	if got := migrated.Definitions["native-model"].AuthorIDs; len(got) != 0 {
		t.Fatalf("marketplace candidates became joint authors: %#v", got)
	}
	if migrated.Report.Missing != 1 {
		t.Fatalf("missing classifications = %d, want 1", migrated.Report.Missing)
	}
}

func TestEmbeddedLegacySchemaMigrationHasReviewedDiffs(t *testing.T) {
	builder, err := NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	wantOfferings := 0
	for _, provider := range catalog.Providers().List() {
		wantOfferings += len(provider.Models)
	}

	migrated, err := MigrateLegacySchema(catalog)
	if err != nil {
		t.Fatalf("MigrateLegacySchema: %v", err)
	}
	if len(migrated.Offerings) != wantOfferings {
		t.Fatalf("offerings = %d, want every legacy provider model %d", len(migrated.Offerings), wantOfferings)
	}
	if len(migrated.Definitions) == 0 || len(migrated.Definitions) > len(migrated.Offerings) {
		t.Fatalf("definition count = %d, offerings = %d", len(migrated.Definitions), len(migrated.Offerings))
	}
	assertMigrationChangesClassified(t, migrated.Report)
	if migrated.Report.Unclassified != 0 {
		t.Fatalf("unclassified changes = %d, want 0", migrated.Report.Unclassified)
	}
	if len(migrated.Offerings) != 516 || len(migrated.Definitions) != 490 ||
		migrated.Report.Exact != 0 || migrated.Report.Defaulted != 1073 ||
		migrated.Report.Conflicts != 23 || migrated.Report.Missing != 81 {
		t.Fatalf("embedded migration review baseline changed: offerings=%d definitions=%d report=%#v",
			len(migrated.Offerings), len(migrated.Definitions), migrated.Report)
	}
	t.Logf("embedded migration: offerings=%d definitions=%d exact=%d defaulted=%d conflicts=%d missing=%d",
		len(migrated.Offerings), len(migrated.Definitions), migrated.Report.Exact,
		migrated.Report.Defaulted, migrated.Report.Conflicts, migrated.Report.Missing)
	var conflicts []string
	var missing []string
	for _, change := range migrated.Report.Changes {
		entry := string(change.Classification) + "|" + string(change.OfferingKey.ProviderID) + "|" + string(change.OfferingKey.ProviderModelID) + "|" + change.Field
		switch change.Classification {
		case MigrationChangeConflict:
			conflicts = append(conflicts, entry)
		case MigrationChangeMissing:
			missing = append(missing, entry)
			if change.Field != "author_ids" || len(migrated.Definitions[ModelDefinitionID(change.OfferingKey.ProviderModelID)].AuthorIDs) != 0 {
				t.Fatalf("missing authorship is not preserved honestly: %s", entry)
			}
		}
	}
	wantConflicts := []string{
		"conflict|google-vertex|gemini-1.5-flash-002|definition",
		"conflict|google-vertex|gemini-1.5-pro-002|definition",
		"conflict|google-vertex|gemini-2.0-flash|definition",
		"conflict|google-vertex|gemini-2.0-flash-001|definition",
		"conflict|google-vertex|gemini-2.0-flash-lite|definition",
		"conflict|google-vertex|gemini-2.0-flash-lite-001|definition",
		"conflict|google-vertex|gemini-2.5-flash|definition",
		"conflict|google-vertex|gemini-2.5-flash-image-preview|definition",
		"conflict|google-vertex|gemini-2.5-flash-lite|definition",
		"conflict|google-vertex|gemini-2.5-flash-lite-preview-06-17|definition",
		"conflict|google-vertex|gemini-2.5-flash-lite-preview-09-2025|definition",
		"conflict|google-vertex|gemini-2.5-flash-preview-05-20|definition",
		"conflict|google-vertex|gemini-2.5-flash-preview-09-2025|definition",
		"conflict|google-vertex|gemini-2.5-pro|definition",
		"conflict|google-vertex|gemini-2.5-pro-preview-05-06|definition",
		"conflict|google-vertex|gemini-2.5-pro-preview-06-05|definition",
		"conflict|google-vertex|gemini-embedding-001|definition",
		"conflict|google-vertex|imagen-3.0-generate-002|definition",
		"conflict|google-vertex|imagen-4.0-fast-generate-001|definition",
		"conflict|google-vertex|imagen-4.0-generate-001|definition",
		"conflict|google-vertex|imagen-4.0-generate-preview-06-06|definition",
		"conflict|google-vertex|imagen-4.0-ultra-generate-001|definition",
		"conflict|google-vertex|imagen-4.0-ultra-generate-preview-06-06|definition",
	}
	if diff := cmp.Diff(wantConflicts, conflicts); diff != "" {
		t.Fatalf("reviewed conflict set changed (-want +got):\n%s", diff)
	}
	missingDigest := sha256.Sum256([]byte(strings.Join(missing, "\n")))
	if got, want := hex.EncodeToString(missingDigest[:]), "6eb0b7b2b831bc887a107b2a706d474b7b0ad79ef755fbbba52fac2ac79dbb50"; got != want {
		t.Fatalf("reviewed missing-authorship set checksum = %s, want %s", got, want)
	}
	for key, offering := range migrated.Offerings {
		if err := offering.Validate(); err != nil {
			t.Fatalf("offering %#v: %v", key, err)
		}
		if _, found := migrated.Definitions[offering.DefinitionID]; !found {
			t.Fatalf("offering %#v references missing definition %q", key, offering.DefinitionID)
		}
	}
}

func TestLegacySchemaMigrationClassifiesSelfLineageWithoutDroppingSource(t *testing.T) {
	legacy := NewEmpty()
	model := legacyMigrationModel("self", 1, "standard")
	root := "self"
	model.Lineage = &ModelLineage{Root: &root}
	if err := legacy.SetProvider(Provider{
		ID: "provider", Name: "Provider", Models: map[string]*Model{"self": model},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	migrated, err := MigrateLegacySchema(legacy)
	if err != nil {
		t.Fatalf("MigrateLegacySchema: %v", err)
	}
	if migrated.Definitions["self"].Lineage.Root != nil {
		t.Fatal("self-referential lineage root survived canonical migration")
	}
	found := false
	for _, change := range migrated.Report.Changes {
		if change.Classification == MigrationChangeConflict && change.Field == "lineage.root" {
			found = true
		}
	}
	if !found {
		t.Fatal("self-lineage normalization was not classified")
	}
}

func assertMigrationChangesClassified(t testing.TB, report LegacySchemaMigrationReport) {
	t.Helper()
	for index, change := range report.Changes {
		switch change.Classification {
		case MigrationChangeExact, MigrationChangeDefaulted, MigrationChangeConflict, MigrationChangeMissing:
		default:
			t.Fatalf("change %d has unknown classification %q", index, change.Classification)
		}
		if change.Field == "" || change.Message == "" {
			t.Fatalf("change %d lacks review detail: %#v", index, change)
		}
	}
}

func legacyMigrationModel(id string, price float64, tier string) *Model {
	return &Model{
		ID:      id,
		Name:    "Shared Model",
		Authors: []Author{{ID: "author", Name: "Author"}},
		Metadata: &ModelMetadata{
			OpenWeights:  true,
			Architecture: &ModelArchitecture{Type: ArchitectureTypeTransformer},
		},
		Features: &ModelFeatures{ToolCalls: true},
		Pricing:  testOfferingPricing(price),
		Limits:   &ModelLimits{ContextWindow: 1000},
		Modes: map[string]ModelMode{
			"fast": {
				Provider: &ModelProviderMode{Body: map[string]any{"service_tier": tier}},
			},
		},
	}
}
