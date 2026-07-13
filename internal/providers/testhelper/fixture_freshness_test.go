package testhelper

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/providerfixture"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/constants"
)

func TestProviderFixtureFreshness(t *testing.T) {
	fixtures, err := filepath.Glob(filepath.Join("..", "*", "testdata", "models_list.json"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("provider fixture gate found no models_list.json fixtures")
	}
	for _, fixture := range fixtures {
		t.Run(filepath.Base(filepath.Dir(filepath.Dir(fixture))), func(t *testing.T) {
			if err := VerifyFixture(fixture, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestProviderFixtureFreshnessRejectsMissingStaleAndTamperedMetadata(t *testing.T) {
	now := time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC)
	fixture := filepath.Join(t.TempDir(), "models_list.json")
	if err := os.WriteFile(fixture, []byte(`{"data":[]}`), constants.SecureFilePermissions); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := VerifyFixture(fixture, now); err == nil {
		t.Fatal("missing metadata passed freshness verification")
	}
	metadata := FixtureMetadata{
		Version: 1, Provider: filepath.Base(filepath.Dir(filepath.Dir(fixture))), FetchedAt: now.Add(-2 * time.Hour),
		SourceRevision: catalogmeta.ObservationRevision{Kind: catalogmeta.ObservationRevisionKindContentDigest, Value: providerfixture.Checksum([]byte(`{"data":[]}`))},
		Payload:        FixturePayload{Path: filepath.Base(fixture), Checksum: providerfixture.Checksum([]byte(`{"data":[]}`))}, MaxAge: time.Hour.String(),
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(providerfixture.MetadataPath(fixture), data, constants.SecureFilePermissions); err != nil {
		t.Fatalf("WriteFile metadata: %v", err)
	}
	if err := VerifyFixture(fixture, now); err == nil {
		t.Fatal("stale metadata passed freshness verification")
	}
	metadata.FetchedAt = now
	metadata.Payload.Checksum = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	data, _ = json.Marshal(metadata)
	if err := os.WriteFile(providerfixture.MetadataPath(fixture), data, constants.SecureFilePermissions); err != nil {
		t.Fatalf("WriteFile tampered metadata: %v", err)
	}
	if err := VerifyFixture(fixture, now); err == nil {
		t.Fatal("tampered payload checksum passed freshness verification")
	}
}

func TestProviderFixtureRefreshCommandFailurePropagatesNonZero(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("repository root: %v", err)
	}
	fakeGo := filepath.Join(t.TempDir(), "fake-go")
	if err := os.WriteFile(fakeGo, []byte("#!/bin/sh\nexit 42\n"), constants.ExecutablePermissions); err != nil {
		t.Fatalf("WriteFile fake go: %v", err)
	}
	command := exec.Command("bash", filepath.Join(root, "scripts", "refresh-provider-testdata.sh"), "openai")
	command.Dir = root
	command.Env = append(os.Environ(), "STARMAP_GO_RUN_BIN="+fakeGo)
	if err := command.Run(); err == nil {
		t.Fatal("provider refresh helper suppressed command failure")
	}
}
