package catalogs

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBootstrapManifestRequiresExactCurrentSchema(t *testing.T) {
	manifest := BootstrapManifest{
		ManifestVersion: CurrentBootstrapManifestVersion,
		GenerationID:    "schema-v2-generation",
		GeneratedAt:     time.Date(2026, 7, 12, 18, 0, 0, 0, time.UTC),
		SchemaVersion:   CurrentCatalogSchemaVersion,
		Payload: PayloadDescriptor{
			Checksum:  "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			SizeBytes: 1,
			MediaType: CatalogPayloadMediaType,
		},
	}
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}
	manifest.SchemaVersion = 1
	if err := manifest.Validate(); err == nil {
		t.Fatal("Validate accepted a prelaunch schema-v1 bootstrap manifest")
	}
	payload, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseBootstrapManifestJSON(payload); err == nil {
		t.Fatal("ParseBootstrapManifestJSON accepted a prelaunch schema-v1 manifest")
	}
}
