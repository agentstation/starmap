package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/bootstrapmanifest"
	"github.com/agentstation/starmap/pkg/constants"
)

func TestScheduledGenerationManifestCommandWritesChangedOnceAndPreservesUnchangedBytes(t *testing.T) {
	catalogDir := filepath.Join("..", "..", "internal", "embedded", "catalog")
	manifestPath := filepath.Join(t.TempDir(), "generation.json")
	now := time.Date(2026, time.July, 10, 16, 0, 0, 0, time.UTC)
	var firstOutput bytes.Buffer
	if err := run([]string{"--catalog-dir", catalogDir, "--output", manifestPath}, &firstOutput, now); err != nil {
		t.Fatalf("run first: %v", err)
	}
	var first bootstrapmanifest.Report
	if err := json.Unmarshal(firstOutput.Bytes(), &first); err != nil {
		t.Fatalf("Unmarshal first: %v", err)
	}
	if !first.Changed || first.GenerationID == "" || first.PayloadChecksum == "" {
		t.Fatalf("first report = %#v", first)
	}
	firstBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile first manifest: %v", err)
	}
	if info, err := os.Stat(manifestPath); err != nil || info.Mode().Perm() != constants.FilePermissions {
		t.Fatalf("manifest permissions = %v, %v", info, err)
	}

	var secondOutput bytes.Buffer
	if err := run([]string{"--catalog-dir", catalogDir, "--output", manifestPath}, &secondOutput, now.Add(24*time.Hour)); err != nil {
		t.Fatalf("run second: %v", err)
	}
	var second bootstrapmanifest.Report
	if err := json.Unmarshal(secondOutput.Bytes(), &second); err != nil {
		t.Fatalf("Unmarshal second: %v", err)
	}
	secondBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile second manifest: %v", err)
	}
	if second.Changed || second.GenerationID != first.GenerationID || !bytes.Equal(secondBytes, firstBytes) {
		t.Fatalf("unchanged rerun report/bytes = %#v/%v", second, bytes.Equal(secondBytes, firstBytes))
	}
}
