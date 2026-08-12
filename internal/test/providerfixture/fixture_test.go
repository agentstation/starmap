package providerfixture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogmeta"
)

func TestProviderFixtureDiscoveryAndVerification(t *testing.T) {
	now := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	writeFixture(t, root, "beta", now, 24*time.Hour, []byte(`{"data":[{"id":"two"}]}`))
	writeFixture(t, root, "alpha", now, 24*time.Hour, []byte(`{"data":[{"id":"one"}]}`))

	fixtures, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(fixtures) != 2 || fixtures[0].Provider != "alpha" || fixtures[1].Provider != "beta" {
		t.Fatalf("fixtures = %#v, want alpha then beta", fixtures)
	}
	for _, fixture := range fixtures {
		if err := fixture.Verify(now); err != nil {
			t.Fatalf("Verify %s: %v", fixture.Provider, err)
		}
		var response struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := fixture.Decode(&response); err != nil {
			t.Fatalf("Decode %s: %v", fixture.Provider, err)
		}
		if len(response.Data) != 1 || response.Data[0].ID == "" {
			t.Fatalf("decoded %s response = %#v", fixture.Provider, response)
		}
	}
}

func TestProviderFixtureRejectsInvalidIdentityFreshnessAndBytes(t *testing.T) {
	now := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)

	t.Run("missing metadata", func(t *testing.T) {
		root := t.TempDir()
		directory := filepath.Join(root, "alpha")
		if err := os.MkdirAll(directory, constants.DirPermissions); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(filepath.Join(directory, fixturePayloadName), []byte(`{"data":[]}`), constants.SecureFilePermissions); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if _, err := Find(root, "alpha"); err == nil {
			t.Fatal("Find accepted a fixture without metadata")
		}
	})

	t.Run("provider mismatch", func(t *testing.T) {
		root := t.TempDir()
		fixture := writeFixture(t, root, "alpha", now, time.Hour, []byte(`{"data":[]}`))
		metadata := readFixtureMetadata(t, fixture.MetadataPath)
		metadata.Provider = "beta"
		writeFixtureMetadata(t, fixture.MetadataPath, metadata)
		if err := fixture.Verify(now); err == nil {
			t.Fatal("Verify accepted mismatched provider metadata")
		}
	})

	t.Run("stale", func(t *testing.T) {
		root := t.TempDir()
		fixture := writeFixture(t, root, "alpha", now.Add(-2*time.Hour), time.Hour, []byte(`{"data":[]}`))
		if err := fixture.Verify(now); err == nil {
			t.Fatal("Verify accepted a stale fixture")
		}
	})

	t.Run("future", func(t *testing.T) {
		root := t.TempDir()
		fixture := writeFixture(t, root, "alpha", now.Add(10*time.Minute), time.Hour, []byte(`{"data":[]}`))
		if err := fixture.Verify(now); err == nil {
			t.Fatal("Verify accepted a future fixture")
		}
	})

	t.Run("tampered payload", func(t *testing.T) {
		root := t.TempDir()
		fixture := writeFixture(t, root, "alpha", now, time.Hour, []byte(`{"data":[]}`))
		if err := os.WriteFile(fixture.PayloadPath, []byte(`{"data":[{"id":"changed"}]}`), constants.SecureFilePermissions); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if err := fixture.Verify(now); err == nil {
			t.Fatal("Verify accepted tampered payload bytes")
		}
	})

	t.Run("unsafe provider", func(t *testing.T) {
		if _, err := Find(t.TempDir(), "../alpha"); err == nil {
			t.Fatal("Find accepted a provider path traversal")
		}
	})
}

func TestProviderFixtureCapturePreservesPolicy(t *testing.T) {
	now := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	fixture := writeFixture(t, root, "alpha", now.Add(-time.Hour), 72*time.Hour, []byte(`{"data":[]}`))

	if err := fixture.Capture([]byte(` {"data":[{"id":"model"}]} `), now); err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if err := fixture.Verify(now); err != nil {
		t.Fatalf("Verify captured fixture: %v", err)
	}
	payload, err := fixture.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	want := "{\n  \"data\": [\n    {\n      \"id\": \"model\"\n    }\n  ]\n}\n"
	if string(payload) != want {
		t.Fatalf("captured payload = %q, want %q", payload, want)
	}
	metadata := readFixtureMetadata(t, fixture.MetadataPath)
	if metadata.MaxAge != (72 * time.Hour).String() {
		t.Fatalf("max_age = %q, want %q", metadata.MaxAge, (72 * time.Hour).String())
	}
	if !metadata.FetchedAt.Equal(now) {
		t.Fatalf("fetched_at = %s, want %s", metadata.FetchedAt, now)
	}
	staged, err := filepath.Glob(filepath.Join(filepath.Dir(fixture.PayloadPath), ".provider-fixture-*.tmp"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(staged) != 0 {
		t.Fatalf("Capture left temporary files: %v", staged)
	}
}

func writeFixture(
	t *testing.T,
	root string,
	provider string,
	fetchedAt time.Time,
	maxAge time.Duration,
	payload []byte,
) Fixture {
	t.Helper()
	directory := filepath.Join(root, provider)
	if err := os.MkdirAll(directory, constants.DirPermissions); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	canonical, err := canonicalJSON(payload)
	if err != nil {
		t.Fatalf("canonicalJSON: %v", err)
	}
	fixture := Fixture{
		Provider: provider, PayloadPath: filepath.Join(directory, fixturePayloadName),
		MetadataPath: filepath.Join(directory, fixtureMetadataName),
	}
	if err := os.WriteFile(fixture.PayloadPath, canonical, constants.SecureFilePermissions); err != nil {
		t.Fatalf("WriteFile payload: %v", err)
	}
	checksum := fixtureChecksum(canonical)
	writeFixtureMetadata(t, fixture.MetadataPath, FixtureMetadata{
		Version: fixtureMetadataVersion, Provider: provider, FetchedAt: fetchedAt,
		SourceRevision: catalogmeta.ObservationRevision{
			Kind: catalogmeta.ObservationRevisionKindContentDigest, Value: checksum,
		},
		Payload: FixturePayload{Path: fixturePayloadName, Checksum: checksum},
		MaxAge:  maxAge.String(),
	})
	return fixture
}

func readFixtureMetadata(t *testing.T, path string) FixtureMetadata {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // Test path is controlled.
	if err != nil {
		t.Fatalf("ReadFile metadata: %v", err)
	}
	var metadata FixtureMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("Unmarshal metadata: %v", err)
	}
	return metadata
}

func writeFixtureMetadata(t *testing.T, path string, metadata FixtureMetadata) {
	t.Helper()
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		t.Fatalf("Marshal metadata: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, constants.SecureFilePermissions); err != nil {
		t.Fatalf("WriteFile metadata: %v", err)
	}
}
