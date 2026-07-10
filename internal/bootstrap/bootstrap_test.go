package bootstrap

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestEmbeddedBootstrapManifestMatchesCanonicalCatalog(t *testing.T) {
	builder, err := catalogs.NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	if _, err := Load(catalog); err != nil {
		t.Fatalf("Load: %v; actual descriptor: %#v", err, catalogs.DescribeCatalogPayload(payload))
	}
}

func TestEmbeddedBootstrapArtifactGenerationIsDeterministic(t *testing.T) {
	first, err := Generation()
	if err != nil {
		t.Fatalf("Generation first: %v", err)
	}
	second, err := Generation()
	if err != nil {
		t.Fatalf("Generation second: %v", err)
	}
	if first.Manifest.GenerationID != second.Manifest.GenerationID ||
		first.Manifest.Payload != second.Manifest.Payload || string(first.Payload) != string(second.Payload) {
		t.Fatalf("embedded generations differ: %#v / %#v", first.Manifest, second.Manifest)
	}
}
