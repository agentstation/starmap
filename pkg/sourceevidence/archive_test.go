package sourceevidence

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestArchiveRetainsOwnerOnlyReplayableEvidenceAcrossReopen(t *testing.T) {
	root := filepath.Join(t.TempDir(), "evidence")
	key := bytes.Repeat([]byte{0x5c}, 32)
	archive, err := NewArchive(root, key, DefaultPolicy())
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}

	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{
		ID: "provider-a", Name: "Provider A",
		Models: map[string]*catalogs.Model{"model-a": {ID: "model-a", Name: "Model A"}},
	}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	observedAt := time.Date(2026, time.July, 10, 14, 0, 0, 0, time.UTC)
	observation, err := sources.NewObservation(sources.ProvidersID, catalog, sources.ObservationMetadata{
		ObservedAt: observedAt, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}
	record, err := Capture(observation)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if err := archive.RetainNormalized(record); err != nil {
		t.Fatalf("RetainNormalized: %v", err)
	}
	raw := RawRecord{
		SourceID: catalogmeta.SourceID("provider-a"), ObservedAt: observedAt,
		MediaType: "application/json", Payload: []byte(`{"models":["model-a"],"secret":"ephemeral"}`),
	}
	if err := archive.RetainRaw(observation.ID, raw); err != nil {
		t.Fatalf("RetainRaw: %v", err)
	}

	reopened, err := NewArchive(root, key, DefaultPolicy())
	if err != nil {
		t.Fatalf("reopen NewArchive: %v", err)
	}
	replayed, err := reopened.ReplayObservation(observation.ID)
	if err != nil {
		t.Fatalf("ReplayObservation: %v", err)
	}
	if replayed.ID != observation.ID || replayed.EvidenceChecksum != observation.EvidenceChecksum {
		t.Fatalf("replayed identity = (%q, %q), want (%q, %q)", replayed.ID, replayed.EvidenceChecksum, observation.ID, observation.EvidenceChecksum)
	}
	opened, err := reopened.OpenRaw(observation.ID, observedAt)
	if err != nil {
		t.Fatalf("OpenRaw: %v", err)
	}
	if !bytes.Equal(opened.Payload, raw.Payload) {
		t.Fatalf("opened payload = %q, want %q", opened.Payload, raw.Payload)
	}

	assertMode(t, root, constants.SecureDirPermissions)
	assertMode(t, filepath.Join(root, "normalized"), constants.SecureDirPermissions)
	assertMode(t, filepath.Join(root, "raw"), constants.SecureDirPermissions)
	assertMode(t, archive.path("normalized", observation.ID), constants.SecureFilePermissions)
	assertMode(t, archive.path("raw", observation.ID), constants.SecureFilePermissions)
	storedRaw, err := os.ReadFile(archive.path("raw", observation.ID))
	if err != nil {
		t.Fatalf("ReadFile raw: %v", err)
	}
	if bytes.Contains(storedRaw, []byte("model-a")) || bytes.Contains(storedRaw, []byte("ephemeral")) {
		t.Fatal("durable raw archive exposes plaintext")
	}
}

func TestArchiveBindsRawEvidenceToObservationAndPurgesAtExpiry(t *testing.T) {
	root := t.TempDir()
	key := bytes.Repeat([]byte{0x6d}, 32)
	archive, err := NewArchive(root, key, DefaultPolicy())
	if err != nil {
		t.Fatalf("NewArchive: %v", err)
	}
	observedAt := time.Date(2026, time.July, 10, 15, 0, 0, 0, time.UTC)
	record := RawRecord{
		SourceID: "provider-a", ObservedAt: observedAt, MediaType: "application/json", Payload: []byte(`{"id":"model-a"}`),
	}
	if err := archive.RetainRaw("observation-a", record); err != nil {
		t.Fatalf("RetainRaw: %v", err)
	}
	data, err := os.ReadFile(archive.path("raw", "observation-a"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var sealed SealedRawRecord
	if err := json.Unmarshal(data, &sealed); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if sealed.ObservationID != "observation-a" {
		t.Fatalf("bound observation = %q, want observation-a", sealed.ObservationID)
	}
	if err := os.WriteFile(archive.path("raw", "observation-b"), data, constants.SecureFilePermissions); err != nil {
		t.Fatalf("Write swapped evidence: %v", err)
	}
	if _, err := archive.OpenRaw("observation-b", observedAt); err == nil {
		t.Fatal("archive accepted raw evidence swapped between observation keys")
	}
	wrongKey, err := NewArchive(root, bytes.Repeat([]byte{0x7e}, 32), DefaultPolicy())
	if err != nil {
		t.Fatalf("NewArchive wrong key: %v", err)
	}
	if _, err := wrongKey.OpenRaw("observation-a", observedAt); err == nil {
		t.Fatal("archive opened raw evidence with the wrong key")
	}

	purged, err := archive.PurgeExpiredRaw(observedAt.Add(DefaultPolicy().RawRetention))
	if err != nil {
		t.Fatalf("PurgeExpiredRaw: %v", err)
	}
	if purged != 2 {
		t.Fatalf("purged = %d, want 2", purged)
	}
	if _, err := os.Stat(archive.path("raw", "observation-a")); !os.IsNotExist(err) {
		t.Fatalf("expired raw file still exists: %v", err)
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%s): %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode(%s) = %#o, want %#o", path, got, want)
	}
}
