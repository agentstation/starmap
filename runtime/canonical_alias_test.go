package runtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

func aliasGeneration(t *testing.T, id string, records ...catalogs.CanonicalAlias) catalogs.Generation {
	t.Helper()
	b := catalogs.NewEmpty()
	if err := b.SetAuthor(catalogs.Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetAuthorModel("author", catalogs.Model{ID: "current", Name: "Current", Authors: []catalogs.Author{{ID: "author", Name: "Author"}}}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetCanonicalAliasRecords(records); err != nil {
		t.Fatal(err)
	}
	catalog, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	generation := catalogs.Generation{Payload: payload, Manifest: catalogs.GenerationManifest{
		ManifestVersion: catalogs.CurrentGenerationManifestVersion, SchemaVersion: catalogs.CurrentCatalogSchemaVersion,
		GenerationID: id, GeneratedAt: at, Payload: catalogs.DescribeCatalogPayload(payload), SyncRunID: id,
		Completeness: catalogs.GenerationCompletenessComplete, ReviewCandidates: []evidence.ReviewCandidate{},
		SourceObservations: []catalogs.SourceObservationLink{{
			Source: "alias_fixture", ObservationID: "observation:" + id, ObservedAt: at,
			Revision:     evidence.ObservationRevision{Kind: evidence.ObservationRevisionKindContentDigest, Value: catalogs.DescribeCatalogPayload(payload).Checksum},
			Completeness: evidence.ObservationCompletenessComplete, Status: evidence.ObservationStatusSucceeded,
			EvidenceChecksum: catalogs.DescribeCatalogPayload(payload).Checksum,
		}},
		ConsumerCompatibility: catalogs.ConsumerCompatibility{MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion, MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion},
		Validation: catalogs.GenerationValidationReport{ValidatorVersion: "alias-fixture/v1", ValidatedAt: at, Status: catalogs.GenerationValidationPassed,
			Checks: []catalogs.GenerationValidationCheck{{Name: "catalog", Status: catalogs.GenerationValidationCheckPassed}}},
	}}
	if _, err := catalogs.DecodeCatalogGeneration(generation); err != nil {
		t.Fatal(err)
	}
	return generation
}

func activeAlias(id string) catalogs.CanonicalAlias {
	return catalogs.CanonicalAlias{ID: catalogs.ModelDefinitionID(id), TargetID: "author/current", PublisherID: "upstream", State: catalogs.CanonicalAliasActive}
}

func aliasRead(generation catalogs.Generation) SourceRead {
	return SourceRead{Changed: true, Generation: generation, PublishedAt: generation.Manifest.GeneratedAt, Health: HealthOK}
}

func TestCanonicalAliasRequiresManifestAndDistinctPublisher(t *testing.T) {
	for _, publisher := range []string{"upstream", "local-runtime", "local-name"} {
		t.Run(publisher, func(t *testing.T) {
			record := activeAlias("author/old")
			record.PublisherID = publisher
			generation := aliasGeneration(t, "rename", record)
			layer := sourceLayer{Payload: generation.Payload}
			if _, err := layer.decodeCatalog(); !errors.IsValidationError(err) {
				t.Errorf("alias without original manifest = %v", err)
			}
			catalog, err := catalogs.DecodeCatalogGeneration(generation)
			if err != nil {
				t.Fatal(err)
			}
			layers := layerSet{publisherID: "local-runtime", publisherAliases: []string{"local-name"}}
			err = layers.validateUpstreamScopePublishers(catalog)
			if publisher == "upstream" && err != nil {
				t.Fatal(err)
			}
			if publisher != "upstream" && !errors.IsConflict(err) {
				t.Fatalf("local alias publisher claim = %v", err)
			}
		})
	}
}

func TestCanonicalAliasSurvivesTimeRestartAndExplicitRemoval(t *testing.T) {
	old, other := activeAlias("author/old"), activeAlias("author/other")
	initial := aliasGeneration(t, "rename", old, other)
	source := newStubSource("canonical-baseline")
	source.replies = []SourceRead{aliasRead(initial)}
	now := initial.Manifest.GeneratedAt
	options := []Option{WithSource(source), WithClock(func() time.Time { return now }),
		WithStateDirectory(privateRuntimeDirectory(t)), WithClientOptions(starmap.WithCatalogStore(storage.NewMemory()))}
	connected := openTestRuntime(t, options...)
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	now = now.Add(365 * 24 * time.Hour)
	assertAliasTarget(t, connected.Catalog(), old.ID)
	target, err := catalogs.NewAliasRemovalTarget(old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := connected.ReplaceRemovalTargets(t.Context(), connected.State(), target); err != nil {
		t.Fatal(err)
	}
	if !connected.Catalog().Removals().ContainsAlias(old.ID) || connected.Catalog().Removals().ContainsAlias(other.ID) {
		t.Fatal("local removal did not remain specific to one alias")
	}
	assertAliasTarget(t, connected.Catalog(), other.ID)
	if err := connected.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := openTestRuntime(t, options...)
	if !restarted.Catalog().Removals().ContainsAlias(old.ID) {
		t.Fatal("restart lost the local alias removal")
	}
	assertAliasTarget(t, restarted.Catalog(), other.ID)
	if _, err := restarted.ReplaceRemovalTargets(t.Context(), restarted.State()); err != nil {
		t.Fatal(err)
	}
	if restarted.Catalog().Removals().ContainsAlias(old.ID) {
		t.Fatal("explicit restore retained the local exclusion")
	}
	assertAliasTarget(t, restarted.Catalog(), old.ID)
	before := restarted.State()
	source.replies = []SourceRead{aliasRead(aliasGeneration(t, "missing-history"))}
	if _, err := restarted.RefreshSource(t.Context()); err == nil {
		t.Fatal("replacement silently dropped accepted rename history")
	}
	if restarted.State().GenerationID != before.GenerationID {
		t.Fatal("failed replacement changed the active generation")
	}
	old.State = catalogs.CanonicalAliasRemoved
	source.replies = []SourceRead{aliasRead(aliasGeneration(t, "explicit-removal", old, other))}
	if _, err := restarted.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.ReplaceRemovalTargets(t.Context(), restarted.State()); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Catalog().FindModel(string(old.ID)); !errors.IsNotFound(err) {
		t.Fatalf("local restore overrode baseline removal: %v", err)
	}
	assertAliasTarget(t, restarted.Catalog(), other.ID)
}

func assertAliasTarget(t *testing.T, catalog *catalogs.Catalog, id catalogs.ModelDefinitionID) {
	t.Helper()
	definition, err := catalog.FindModel(string(id))
	if err != nil || definition.ID != "author/current" || len(catalog.Definitions()) != 1 {
		t.Fatalf("alias %s = %s, %v", id, definition.ID, err)
	}
}

func TestCanonicalAliasRejectsLegacySourceReplacement(t *testing.T) {
	previous := aliasGeneration(t, "rename", activeAlias("author/old"))
	next := aliasGeneration(t, "legacy")
	var payload map[string]any
	if err := json.Unmarshal(next.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	payload["schema_version"] = 8
	var err error
	next.Payload, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	next.Manifest.SchemaVersion = 8
	next.Manifest.ConsumerCompatibility = catalogs.ConsumerCompatibility{MinSchemaVersion: 8, MaxSchemaVersion: 8}
	next.Manifest.Payload = catalogs.DescribeCatalogPayload(next.Payload)
	source := newStubSource("canonical-baseline")
	source.replies = []SourceRead{aliasRead(previous), aliasRead(next)}
	connected := openTestRuntime(t, WithSource(source), WithStateDirectory(privateRuntimeDirectory(t)))
	if _, err := connected.RefreshSource(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := connected.RefreshSource(t.Context()); err == nil {
		t.Fatal("legacy replacement dropped accepted rename history")
	}
	assertAliasTarget(t, connected.Catalog(), "author/old")
}
