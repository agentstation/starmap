package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestArtifactReleaseCommandStagesImmutableEmbeddedGeneration(t *testing.T) {
	root := t.TempDir()
	var firstOutput bytes.Buffer
	if err := run([]string{"--output-dir", root}, &firstOutput); err != nil {
		t.Fatalf("run first: %v", err)
	}
	var first releaseReport
	if err := json.Unmarshal(firstOutput.Bytes(), &first); err != nil {
		t.Fatalf("Unmarshal report: %v", err)
	}
	if first.GenerationID == "" || len(first.Files) != 3 || len(first.ArchiveChecksum) != len("sha256:")+64 {
		t.Fatalf("report = %#v", first)
	}
	for _, path := range first.Files {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("release asset %q: %v", path, err)
		}
	}

	var secondOutput bytes.Buffer
	if err := run([]string{"--output-dir", root}, &secondOutput); err != nil {
		t.Fatalf("run idempotent retry: %v", err)
	}
	if secondOutput.String() != firstOutput.String() {
		t.Fatalf("retry report changed:\nfirst %s\nsecond %s", firstOutput.String(), secondOutput.String())
	}
}
