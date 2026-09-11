package permission

import (
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/provenance"
)

func changeOriginCatalog(t *testing.T, input catalogs.Generation, change func(*catalogs.Builder) error) catalogs.Generation {
	t.Helper()
	catalog, err := catalogs.DecodeCatalogGeneration(input)
	if err != nil {
		t.Fatal(err)
	}
	builder, err := catalogs.NewBuilderFrom(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := change(builder); err != nil {
		t.Fatal(err)
	}
	output := input.Copy()
	output.Payload, err = catalogs.EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	output.Manifest.Payload = catalogs.DescribeCatalogPayload(output.Payload)
	return output
}

func TestPrepareGenerationRevisionExcludesPublicationMetadata(t *testing.T) {
	input := originInput(t, true)
	config := GenerationConfig{AuthorityID: "enterprise", PolicyID: "production", Sequence: 1}
	first := prepareOrigin(t, input, config)
	for _, scenario := range []struct {
		name   string
		change func(*catalogs.Generation, *GenerationConfig)
	}{
		{"sequence", func(_ *catalogs.Generation, c *GenerationConfig) { c.Sequence++ }},
		{"source generation", func(g *catalogs.Generation, _ *GenerationConfig) { g.Manifest.GenerationID = "another-source" }},
		{"publication time", func(g *catalogs.Generation, _ *GenerationConfig) {
			g.Manifest.GeneratedAt = g.Manifest.GeneratedAt.Add(time.Second)
		}},
		{"validator", func(g *catalogs.Generation, _ *GenerationConfig) {
			g.Manifest.Validation.ValidatorVersion = "validator/v2"
		}},
		{"provenance", func(g *catalogs.Generation, _ *GenerationConfig) {
			*g = changeOriginCatalog(t, *g, func(b *catalogs.Builder) error {
				b.SetProvenance(provenance.Map{"providers.provider.Name": {{Field: "Name", Value: "Provider"}}})
				return nil
			})
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			nextInput, nextConfig := input.Copy(), config
			scenario.change(&nextInput, &nextConfig)
			next := prepareOrigin(t, nextInput, nextConfig)
			if next.Manifest.AuthorityHead.RequiredPermissionRevision != first.Manifest.AuthorityHead.RequiredPermissionRevision {
				t.Fatal("publication metadata changed required catalog semantics")
			}
			if next.Manifest.GenerationID == first.Manifest.GenerationID {
				t.Fatal("changed immutable publication retained its generation identity")
			}
		})
	}
}

func TestPrepareGenerationRevisionBindsAuthorityAndPolicy(t *testing.T) {
	input := originInput(t, true)
	config := GenerationConfig{AuthorityID: "enterprise", PolicyID: "production", Sequence: 1}
	first := prepareOrigin(t, input, config)
	for _, nextConfig := range []GenerationConfig{
		{AuthorityID: "another-enterprise", PolicyID: config.PolicyID, Sequence: 1},
		{AuthorityID: config.AuthorityID, PolicyID: "another-policy", Sequence: 1},
	} {
		next := prepareOrigin(t, input, nextConfig)
		if next.Manifest.AuthorityHead.RequiredPermissionRevision == first.Manifest.AuthorityHead.RequiredPermissionRevision || next.Manifest.GenerationID == first.Manifest.GenerationID {
			t.Fatal("authority or policy identity did not change both publication identities")
		}
	}
}

func TestPrepareGenerationRevisionBindsRemovalAndAliasPolicy(t *testing.T) {
	input := originInput(t, true)
	config := GenerationConfig{AuthorityID: "enterprise", PolicyID: "production", Sequence: 1}
	first := prepareOrigin(t, input, config)
	for _, scenario := range []struct {
		name   string
		change func(*catalogs.Builder) error
	}{
		{"canonical removal", func(b *catalogs.Builder) error {
			return b.SetRemovalPolicies([]catalogs.CatalogRemovalPolicy{{PublisherID: "enterprise", Targets: []catalogs.CatalogRemovalTarget{
				{Kind: catalogs.CatalogRemovalCanonical, DefinitionID: "author/model"},
			}}})
		}},
		{"active alias", func(b *catalogs.Builder) error {
			return b.SetCanonicalAliasRecords([]catalogs.CanonicalAlias{{ID: "author/old", TargetID: "author/model", PublisherID: "enterprise", State: catalogs.CanonicalAliasActive}})
		}},
		{"removed alias", func(b *catalogs.Builder) error {
			return b.SetCanonicalAliasRecords([]catalogs.CanonicalAlias{{ID: "author/old", TargetID: "author/model", PublisherID: "enterprise", State: catalogs.CanonicalAliasRemoved}})
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			changed := changeOriginCatalog(t, input, scenario.change)
			next := prepareOrigin(t, changed, config)
			if next.Manifest.AuthorityHead.RequiredPermissionRevision == first.Manifest.AuthorityHead.RequiredPermissionRevision {
				t.Fatal("changed catalog policy reused the original permission revision")
			}
		})
	}
}

func TestPrepareGenerationRetainsAliasUntilExplicitRemoval(t *testing.T) {
	input := originInput(t, true)
	alias := catalogs.CanonicalAlias{ID: "author/old", TargetID: "author/model", PublisherID: "enterprise", State: catalogs.CanonicalAliasActive}
	input = changeOriginCatalog(t, input, func(b *catalogs.Builder) error { return b.SetCanonicalAliasRecords([]catalogs.CanonicalAlias{alias}) })
	config := GenerationConfig{AuthorityID: "enterprise", PolicyID: "production", Sequence: 1}
	first := prepareOrigin(t, input, config)
	input.Manifest.GeneratedAt = input.Manifest.GeneratedAt.AddDate(20, 0, 0)
	config.Sequence++
	later := prepareOrigin(t, input, config)
	if later.Manifest.AuthorityHead.RequiredPermissionRevision != first.Manifest.AuthorityHead.RequiredPermissionRevision {
		t.Fatal("elapsed publication time changed retained alias permission")
	}
	catalog, err := catalogs.DecodeCatalogGeneration(later)
	if err != nil {
		t.Fatal(err)
	}
	if definition, err := catalog.FindModel("author/old"); err != nil || definition.ID != "author/model" {
		t.Fatalf("retained alias no longer resolves: %v", err)
	}
	alias.State = catalogs.CanonicalAliasRemoved
	input = changeOriginCatalog(t, input, func(b *catalogs.Builder) error { return b.SetCanonicalAliasRecords([]catalogs.CanonicalAlias{alias}) })
	config.Sequence++
	removed := prepareOrigin(t, input, config)
	if removed.Manifest.AuthorityHead.RequiredPermissionRevision == later.Manifest.AuthorityHead.RequiredPermissionRevision {
		t.Fatal("explicit alias withdrawal reused the active alias revision")
	}
	removedCatalog, err := catalogs.DecodeCatalogGeneration(removed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := removedCatalog.FindModel("author/old"); err == nil {
		t.Fatal("explicitly removed alias still resolves")
	}
	if _, err := removedCatalog.FindModel("author/model"); err != nil {
		t.Fatal("alias removal also removed its canonical target")
	}
	input = changeOriginCatalog(t, input, func(b *catalogs.Builder) error { return b.SetCanonicalAliasRecords(nil) })
	config.Sequence++
	replacement := prepareOrigin(t, input, config)
	if replacement.Manifest.AuthorityHead.RequiredPermissionRevision == later.Manifest.AuthorityHead.RequiredPermissionRevision {
		t.Fatal("replacement baseline omitted the alias without changing its active permission revision")
	}
}
