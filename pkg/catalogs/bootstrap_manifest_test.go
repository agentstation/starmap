package catalogs

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBootstrapManifestRequiresSemanticAndExactPayloadIdentities(t *testing.T) {
	catalog, err := NewEmpty().Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	payload, err := EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	semanticChecksum, err := CatalogSemanticChecksum(catalog)
	if err != nil {
		t.Fatalf("CatalogSemanticChecksum: %v", err)
	}
	valid := BootstrapManifest{
		ManifestVersion:  CurrentBootstrapManifestVersion,
		GenerationID:     "bootstrap-generation",
		GeneratedAt:      time.Date(2026, time.July, 29, 23, 0, 0, 0, time.UTC),
		SchemaVersion:    CurrentCatalogSchemaVersion,
		SemanticChecksum: semanticChecksum,
		Payload:          DescribeCatalogPayload(payload),
	}
	data, err := json.Marshal(valid)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	parsed, err := ParseBootstrapManifestJSON(data)
	if err != nil {
		t.Fatalf("ParseBootstrapManifestJSON: %v", err)
	}
	if parsed != valid {
		t.Fatalf("parsed = %#v, want %#v", parsed, valid)
	}

	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	delete(document, "semantic_checksum")
	missingSemantic, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("Marshal missing semantic checksum: %v", err)
	}
	if _, err := ParseBootstrapManifestJSON(missingSemantic); err == nil {
		t.Fatal("ParseBootstrapManifestJSON accepted missing semantic checksum")
	}

	invalid := valid
	invalid.SemanticChecksum = invalid.Payload.Checksum[:len("sha256:")+63]
	invalidData, err := json.Marshal(invalid)
	if err != nil {
		t.Fatalf("Marshal invalid: %v", err)
	}
	if _, err := ParseBootstrapManifestJSON(invalidData); err == nil {
		t.Fatal("ParseBootstrapManifestJSON accepted malformed semantic checksum")
	}
}
