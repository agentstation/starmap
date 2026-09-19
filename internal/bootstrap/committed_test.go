package bootstrap

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/agentstation/starmap/internal/bootstrap/manifest"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
)

func TestEmbeddedGenerationPreservesCommittedMembershipEvidence(t *testing.T) {
	catalog, bootstrap, original := committedBootstrapFixture(t)
	data, err := json.Marshal(original.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{catalogs.BootstrapGenerationManifestFilename: &fstest.MapFile{Data: data}}
	actual, err := buildGeneration(fsys, catalog, bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual.Manifest, original.Manifest) || !bytes.Equal(actual.Payload, original.Payload) {
		t.Fatal("bootstrap replaced committed source evidence or catalog bytes")
	}
	if _, err := catalogs.DecodeCatalogGeneration(actual); err != nil {
		t.Fatal(err)
	}
}

func TestEmbeddedGenerationRejectsMissingOrChangedCommittedEvidence(t *testing.T) {
	catalog, bootstrap, original := committedBootstrapFixture(t)
	for _, kind := range []string{"missing", "malformed", "identity", "time", "checksum", "membership"} {
		t.Run(kind, func(t *testing.T) {
			generation := original.Copy()
			switch kind {
			case "identity":
				generation.Manifest.GenerationID = "another-generation"
			case "time":
				generation.Manifest.GeneratedAt = generation.Manifest.GeneratedAt.Add(time.Second)
			case "checksum":
				generation.Manifest.Payload.SizeBytes++
			case "membership":
				generation.Manifest.SourceObservations[0].ObservationID = "unrelated-observation"
			}
			data, err := json.Marshal(generation.Manifest)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "malformed" {
				data = []byte("{")
			}
			fsys := fstest.MapFS{}
			if kind != "missing" {
				fsys[catalogs.BootstrapGenerationManifestFilename] = &fstest.MapFile{Data: data}
			}
			if _, err := buildGeneration(fsys, catalog, bootstrap); err == nil {
				t.Fatal("bootstrap accepted incomplete or changed source evidence")
			}
		})
	}
}

func committedBootstrapFixture(t *testing.T) (*catalogs.Catalog, catalogs.BootstrapManifest, catalogs.Generation) {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "provider", Name: "Provider"}); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	bootstrap, _, err := manifest.Derive(catalog, nil, time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	generation, err := buildGeneration(fstest.MapFS{}, catalog, bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	generation.Manifest.SourceObservations[0].Source = evidence.ProvidersID
	generation.Manifest.SourceObservations[0].ObservationID = "provider-observation"
	observation := generation.Manifest.SourceObservations[0]
	if err := builder.SetMembershipScopes([]catalogs.ProviderMembershipScope{{
		PublisherID: "publisher", BindingID: "binding", BindingRevision: "1", ProviderID: "provider",
		Region: "default-endpoint", APISurface: "models", Authority: catalogs.MembershipScopeAuthority, Public: true,
		Inventory: &catalogs.MembershipInventory{ObservationID: observation.ObservationID, ObservedAt: observation.ObservedAt, ModelIDs: []string{}},
	}}); err != nil {
		t.Fatal(err)
	}
	catalog, err = builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	generation.Payload, err = catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatal(err)
	}
	generation.Manifest.Payload = catalogs.DescribeCatalogPayload(generation.Payload)
	// Preserve a distinct source receipt so missing metadata cannot use the legacy fallback.
	generation.Manifest.GenerationID = "committed-provider-inventory"
	bootstrap, _, err = manifest.DeriveCommitted(catalog, generation, nil)
	if err != nil {
		t.Fatal(err)
	}
	return catalog, bootstrap, generation
}
