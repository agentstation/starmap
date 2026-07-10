package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/sources/modelsdev"
)

func TestCatalogGenerationToolingPromotionCommand(t *testing.T) {
	input := filepath.Join("..", "..", "internal", "embedded", "sources", "models.dev", "api.json")
	destination := filepath.Join(t.TempDir(), "api.json")
	var output bytes.Buffer
	if err := run([]string{"--input", input, "--output", destination}, &output); err != nil {
		t.Fatalf("run: %v", err)
	}
	var report modelsdev.APIPromotion
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("Unmarshal report: %v", err)
	}
	if report.Checksum == "" || report.SizeBytes == 0 || report.ProviderCount == 0 || report.ModelCount == 0 {
		t.Fatalf("report = %#v", report)
	}
	if _, err := os.Stat(destination); err != nil {
		t.Fatalf("promoted destination: %v", err)
	}
}

func TestCatalogGenerationToolingPromotionCommandRejectsMissingPaths(t *testing.T) {
	if err := run(nil, &bytes.Buffer{}); err == nil {
		t.Fatal("run accepted missing paths")
	}
}
