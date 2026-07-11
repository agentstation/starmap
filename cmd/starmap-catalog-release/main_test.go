package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogartifact"
	"github.com/agentstation/starmap/pkg/constants"
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

func TestArtifactReleaseCommandVerifiesDownloadedReleaseSet(t *testing.T) {
	root := t.TempDir()
	var stagedOutput bytes.Buffer
	if err := run([]string{"--output-dir", root}, &stagedOutput); err != nil {
		t.Fatalf("stage release: %v", err)
	}
	var staged releaseReport
	if err := json.Unmarshal(stagedOutput.Bytes(), &staged); err != nil {
		t.Fatalf("Unmarshal staged report: %v", err)
	}

	var verifiedOutput bytes.Buffer
	if err := run([]string{"--verify-dir", staged.Directory}, &verifiedOutput); err != nil {
		t.Fatalf("verify release: %v", err)
	}
	var verified releaseReport
	if err := json.Unmarshal(verifiedOutput.Bytes(), &verified); err != nil {
		t.Fatalf("Unmarshal verified report: %v", err)
	}
	if verified.GenerationID != staged.GenerationID || verified.ArchiveChecksum != staged.ArchiveChecksum || len(verified.Files) != 3 {
		t.Fatalf("verified report = %#v, staged = %#v", verified, staged)
	}
}

func TestArtifactReleaseCommandRejectsTamperedReleaseSet(t *testing.T) {
	root := t.TempDir()
	var output bytes.Buffer
	if err := run([]string{"--output-dir", root}, &output); err != nil {
		t.Fatalf("stage release: %v", err)
	}
	var staged releaseReport
	if err := json.Unmarshal(output.Bytes(), &staged); err != nil {
		t.Fatalf("Unmarshal staged report: %v", err)
	}
	archivePath := filepath.Join(staged.Directory, catalogartifact.Filename)
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("Read archive: %v", err)
	}
	if err := os.WriteFile(archivePath, append(archive, 'x'), constants.FilePermissions); err != nil {
		t.Fatalf("Tamper archive: %v", err)
	}
	if err := run([]string{"--verify-dir", staged.Directory}, io.Discard); err == nil {
		t.Fatal("verification accepted a tampered archive")
	}
}

func TestArtifactReleaseCommandRejectsTamperedDetachedStatement(t *testing.T) {
	root := t.TempDir()
	var output bytes.Buffer
	if err := run([]string{"--output-dir", root}, &output); err != nil {
		t.Fatalf("stage release: %v", err)
	}
	var staged releaseReport
	if err := json.Unmarshal(output.Bytes(), &staged); err != nil {
		t.Fatalf("Unmarshal staged report: %v", err)
	}
	statementPath := filepath.Join(staged.Directory, catalogartifact.AttestationFilename)
	statement, err := os.ReadFile(statementPath)
	if err != nil {
		t.Fatalf("Read detached statement: %v", err)
	}
	if err := os.WriteFile(statementPath, append(statement, 'x'), constants.FilePermissions); err != nil {
		t.Fatalf("Tamper detached statement: %v", err)
	}
	if err := run([]string{"--verify-dir", staged.Directory}, io.Discard); err == nil {
		t.Fatal("verification accepted a tampered detached statement")
	}
}
