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
	if definitions := catalog.Definitions(); len(definitions) == 0 {
		t.Fatal("embedded catalog published no canonical definitions")
	}
	offeringCount := 0
	for _, provider := range catalog.Providers().List() {
		offerings, err := catalog.ProviderOfferings(provider.ID)
		if err != nil {
			t.Fatalf("ProviderOfferings(%s): %v", provider.ID, err)
		}
		for _, offering := range offerings {
			if err := offering.Validate(); err != nil {
				t.Fatalf("Offering(%s/%s): %v", offering.ProviderID, offering.ProviderModelID, err)
			}
		}
		offeringCount += len(offerings)
	}
	if offeringCount == 0 {
		t.Fatal("embedded catalog published no provider offerings")
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
