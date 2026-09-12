package storage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agentstation/starmap/internal/privatefiles"
)

func TestFilesystemRetentionProtectsLeasesAcrossInstances(t *testing.T) {
	root := filepath.Join(t.TempDir(), "store")
	first, err := NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	retaining, ok := any(first).(RetainingStore)
	if !ok {
		t.Fatal("filesystem catalog store does not support generation retention")
	}
	previous := ""
	for _, id := range []string{"baseline", "expired", "leased", "current"} {
		if err := first.Commit(t.Context(), testGeneration(id, id), previous); err != nil {
			t.Fatal(err)
		}
		previous = id
	}
	second, err := NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	reader := any(second).(RetainingStore)
	got, release, err := reader.AcquireGeneration(t.Context(), "leased")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = release() })
	got.Payload[0] = '!'
	request := RetentionRequest{ExpectedGenerationID: "current", RequiredGenerationIDs: []string{"baseline"}, MaxGenerations: 2, MaxBytes: 1 << 20}
	dry := request
	dry.DryRun = true
	preview, err := retaining.Collect(t.Context(), dry)
	if err != nil || !reflect.DeepEqual(preview.Candidates, []string{"expired"}) || preview.After != preview.Before || len(preview.Removed) != 0 {
		t.Fatalf("dry run: %+v %v", preview, err)
	}
	report, err := retaining.Collect(t.Context(), request)
	if err != nil || !reflect.DeepEqual(report.Removed, []string{"expired"}) || !report.OverLimit {
		t.Fatalf("leased collection: %+v %v", report, err)
	}
	if _, err := second.Get(t.Context(), "leased"); err != nil {
		t.Fatal(err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	report, err = retaining.Collect(t.Context(), request)
	if err != nil || !reflect.DeepEqual(report.Removed, []string{"leased"}) || report.OverLimit {
		t.Fatalf("released collection: %+v %v", report, err)
	}
}

func TestFilesystemRetentionPreservesUnknownFiles(t *testing.T) {
	first, err := NewFilesystem(filepath.Join(t.TempDir(), "store"))
	if err != nil {
		t.Fatal(err)
	}
	retaining, ok := any(first).(RetainingStore)
	if !ok {
		t.Fatal("filesystem catalog store does not support generation retention")
	}
	for index, id := range []string{"old", "current"} {
		previous := ""
		if index > 0 {
			previous = "old"
		}
		if err := first.Commit(t.Context(), testGeneration(id, id), previous); err != nil {
			t.Fatal(err)
		}
	}
	unknown := filepath.Join(first.generationDir("old"), "operator-note.txt")
	if err := os.WriteFile(unknown, []byte("keep"), privatefiles.FileMode); err != nil {
		t.Fatal(err)
	}
	_, err = retaining.Collect(t.Context(), RetentionRequest{ExpectedGenerationID: "current", MaxGenerations: 1, MaxBytes: 1 << 20})
	if err == nil {
		t.Fatal("collection accepted unrecognized generation contents")
	}
	if data, err := os.ReadFile(unknown); err != nil || string(data) != "keep" {
		t.Fatalf("unknown file changed: %q %v", data, err)
	}
	if _, err := first.Get(t.Context(), "old"); err != nil {
		t.Fatal(err)
	}
}
