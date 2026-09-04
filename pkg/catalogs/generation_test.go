package catalogs

import (
	"os"
	"testing"
)

func TestGenerationCopyOwnsManifestAndPayload(t *testing.T) {
	t.Parallel()

	generation := loadGenerationFixture(t)
	copy := generation.Copy()
	copy.Payload[0] ^= 0xff
	copy.Manifest.Validation.Checks[0].Name = "changed"
	copy.Manifest.SourceObservations[0].ObservationID = "changed"

	if string(generation.Payload) == string(copy.Payload) {
		t.Fatal("payload mutation did not change the copy")
	}
	if generation.Manifest.Validation.Checks[0].Name == "changed" {
		t.Fatal("validation-check mutation reached the original")
	}
	if generation.Manifest.SourceObservations[0].ObservationID == "changed" {
		t.Fatal("observation mutation reached the original")
	}
}

func TestGenerationValidateBindsManifestToPayload(t *testing.T) {
	t.Parallel()

	generation := loadGenerationFixture(t)
	if err := generation.Validate(); err != nil {
		t.Fatalf("Validate fixture: %v", err)
	}
	generation.Payload = append(generation.Payload, 'x')
	if err := generation.Validate(); err == nil {
		t.Fatal("Validate accepted payload bytes that do not match the manifest")
	}
}

func loadGenerationFixture(t *testing.T) Generation {
	t.Helper()
	payload, err := os.ReadFile("testdata/generation/catalog.json")
	if err != nil {
		t.Fatalf("Read payload fixture: %v", err)
	}
	return Generation{Manifest: loadGenerationManifestFixture(t), Payload: payload}
}

func TestGenerationSemanticChecksumExcludesProvenance(t *testing.T) {
	t.Parallel()

	catalog, err := NewEmpty().Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	payload, err := EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	generation := Generation{Manifest: loadGenerationManifestFixture(t), Payload: payload}
	generation.Manifest.Payload = DescribeCatalogPayload(payload)
	if err := generation.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	checksum, err := generation.SemanticChecksum()
	if err != nil {
		t.Fatalf("SemanticChecksum: %v", err)
	}
	want, err := CatalogSemanticChecksum(catalog)
	if err != nil {
		t.Fatalf("CatalogSemanticChecksum: %v", err)
	}
	if checksum != want {
		t.Fatalf("SemanticChecksum = %s, want %s", checksum, want)
	}
	if checksum == generation.Manifest.Payload.Checksum {
		t.Fatal("the semantic checksum must not equal the exact payload checksum")
	}

	generation.Payload = []byte("{")
	if _, err := generation.SemanticChecksum(); err == nil {
		t.Fatal("SemanticChecksum accepted a payload that does not decode")
	}
}
