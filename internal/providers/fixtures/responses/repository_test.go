package responses

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/constants"
)

func TestProviderFixtureFreshness(t *testing.T) {
	fixtures, err := filepath.Glob(filepath.Join("*", "*", "models_list.json"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("provider response fixture gate found no models_list.json fixtures")
	}
	for _, fixture := range fixtures {
		t.Run(ProviderFromPath(fixture)+"/"+SourceFromPath(fixture), func(t *testing.T) {
			if err := Verify(fixture, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestProviderFixtureFreshnessRejectsMissingStaleAndTamperedMetadata(t *testing.T) {
	now := time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC)
	fixture := filepath.Join(t.TempDir(), "provider", "source", "models_list.json")
	payload := []byte(`{"data":[]}`)
	if err := os.MkdirAll(filepath.Dir(fixture), constants.DirPermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture, payload, constants.SecureFilePermissions); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := Verify(fixture, now); err == nil {
		t.Fatal("missing metadata passed freshness verification")
	}
	checksum := Checksum(payload)
	metadata := Metadata{
		Version: 1, Provider: "provider", Source: "source", FetchedAt: now.Add(-2 * time.Hour),
		SourceRevision: catalogmeta.ObservationRevision{Kind: catalogmeta.ObservationRevisionKindContentDigest, Value: checksum},
		Payload:        Payload{Path: filepath.Base(fixture), Checksum: checksum}, MaxAge: time.Hour.String(),
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(MetadataPath(fixture), data, constants.SecureFilePermissions); err != nil {
		t.Fatalf("WriteFile metadata: %v", err)
	}
	if err := Verify(fixture, now); err == nil {
		t.Fatal("stale metadata passed freshness verification")
	}
	metadata.FetchedAt = now
	metadata.Payload.Checksum = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	data, err = json.Marshal(metadata)
	if err != nil {
		t.Fatalf("Marshal tampered metadata: %v", err)
	}
	if err := os.WriteFile(MetadataPath(fixture), data, constants.SecureFilePermissions); err != nil {
		t.Fatalf("WriteFile tampered metadata: %v", err)
	}
	if err := Verify(fixture, now); err == nil {
		t.Fatal("tampered payload checksum passed freshness verification")
	}
}
