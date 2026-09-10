package privatefiles

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateRecordsPreserveOtherFilesAndBoundReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records")
	directory, err := NewDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(path, "source.json.tmp")
	if err := os.WriteFile(legacy, []byte("operator scratch"), FileMode); err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{[]byte("first"), []byte("second")} {
		if err := directory.WriteFile("source.json", data, ".layer-"); err != nil {
			t.Fatal(err)
		}
		got, err := directory.ReadFile("source.json", int64(len(data)))
		if err != nil || !bytes.Equal(got, data) {
			t.Fatal("private record did not survive publication", err)
		}
		if _, err := directory.ReadFile("source.json", int64(len(data)-1)); err == nil {
			t.Fatal("record exceeded its read bound")
		}
	}
	got, err := os.ReadFile(legacy)
	if err != nil || string(got) != "operator scratch" {
		t.Fatal("publication replaced the legacy temporary path", err)
	}
	entries, err := directory.ReadDir()
	if err != nil || len(entries) != 2 {
		t.Fatal("private publication left temporary files", err)
	}
}

func TestPrivateDirectoryRefusesPathReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records")
	directory, err := NewDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := directory.WriteFile("source.json", []byte("retained"), ".layer-"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".original"); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDirectory(path); err != nil {
		t.Fatal(err)
	}
	if _, err := directory.ReadFile("source.json", 100); err == nil {
		t.Fatal("read accepted another directory")
	}
	if err := directory.WriteFile("source.json", []byte("new"), ".layer-"); err == nil {
		t.Fatal("write accepted another directory")
	}
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 0 {
		t.Fatal("replacement directory changed", err)
	}
	got, err := os.ReadFile(filepath.Join(path+".original", "source.json"))
	if err != nil || string(got) != "retained" {
		t.Fatal("original record changed", err)
	}
}

func TestPrivateDirectoryPassiveBindingAndChildCreation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records")
	if _, err := ExistingDirectory(path); !os.IsNotExist(err) {
		t.Fatal("passive binding did not report absence", err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("passive binding created its directory", err)
	}
	directory, err := NewDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := directory.ExistingChild("providers"); !os.IsNotExist(err) {
		t.Fatal("passive child binding did not report absence", err)
	}
	child, err := directory.Child("providers")
	if err != nil {
		t.Fatal(err)
	}
	if err := child.WriteFile("provider.json", []byte("record"), ".layer-"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../escape", "a/b", `a\b`, "stream:alternate", "trailing.", "."} {
		if err := directory.WriteFile(name, []byte("invalid"), ".layer-"); err == nil {
			t.Errorf("invalid record name accepted: %s", name)
		}
	}
}

func TestPrivateDirectoryAbsenceIsNotAnAbsentRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records")
	directory, err := NewDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := directory.ReadFile("missing.json", 100); !os.IsNotExist(err) {
		t.Fatal("missing record did not report absence", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := directory.ReadFile("missing.json", 100); err == nil || os.IsNotExist(err) {
		t.Fatal("lost directory appeared to be an absent record", err)
	}
}

func TestPrivatePublicationPreservesCompetingFiles(t *testing.T) {
	for _, target := range []string{"destination", "staging"} {
		t.Run(target, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "records")
			directory, err := NewDirectory(path)
			if err != nil {
				t.Fatal(err)
			}
			var competing string
			directory.beforePublish = func(stage string) error {
				competing = filepath.Join(path, "source.json")
				if target == "staging" {
					competing = filepath.Join(path, stage)
					if err := os.Rename(competing, competing+".original"); err != nil {
						return err
					}
				}
				return os.WriteFile(competing, []byte("operator content"), FileMode)
			}
			if err := directory.WriteFile("source.json", []byte("candidate"), ".layer-"); err == nil {
				t.Fatal("publication accepted a competing file")
			}
			data, err := os.ReadFile(competing)
			if err != nil || string(data) != "operator content" {
				t.Fatal("publication or cleanup changed the competing file", err)
			}
		})
	}
}

func TestPrivatePublicationCancellationPreservesDestination(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "absent"
		if existing {
			name = "existing"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "records")
			directory, err := NewDirectory(path)
			if err != nil {
				t.Fatal(err)
			}
			if existing {
				if err := directory.WriteFile("current", []byte("old"), ".current-"); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			directory.beforePublish = func(string) error { cancel(); return nil }
			if err := directory.WriteFileContext(ctx, "current", []byte("new"), ".current-"); !errors.Is(err, context.Canceled) {
				t.Fatalf("write = %v", err)
			}
			data, err := directory.ReadFile("current", 10)
			if existing && (err != nil || string(data) != "old") {
				t.Fatalf("old destination lost: %q %v", data, err)
			}
			if !existing && !os.IsNotExist(err) {
				t.Fatalf("absent destination published: %q %v", data, err)
			}
			entries, err := os.ReadDir(path)
			want := 0
			if existing {
				want = 1
			}
			if err != nil || len(entries) != want {
				t.Fatalf("staging survived: %v %v", entries, err)
			}
		})
	}
}

func TestPrivatePublicationPreservesModifiedStage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records")
	directory, err := NewDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	var modified string
	directory.beforePublish = func(stage string) error {
		modified = filepath.Join(path, stage)
		return os.WriteFile(modified, []byte("operator change"), FileMode)
	}
	if err := directory.WriteFile("current", []byte("candidate"), ".current-"); err == nil {
		t.Fatal("published modified bytes")
	}
	data, err := os.ReadFile(modified)
	if err != nil || string(data) != "operator change" {
		t.Fatalf("cleanup removed modified file: %q %v", data, err)
	}
}

func TestPrivateRecordCreatePreservesExistingAndConcurrentFiles(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "concurrent", true: "existing"}[existing], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "exclusive")
			directory, err := NewDirectory(path)
			if err != nil {
				t.Fatal(err)
			}
			other := []byte("other publication")
			if existing {
				if err := directory.WriteFile("authority.json", other, ".test-"); err != nil {
					t.Fatal(err)
				}
			} else {
				directory.beforePublish = func(string) error {
					return os.WriteFile(filepath.Join(path, "authority.json"), other, FileMode)
				}
			}
			if err := directory.WriteFileIfAbsentContext(t.Context(), "authority.json", []byte("candidate"), ".test-"); err == nil {
				t.Fatal("replaced an existing publication")
			}
			data, err := directory.ReadFile("authority.json", 100)
			if err != nil || !bytes.Equal(data, other) {
				t.Fatalf("data=%q error=%v", data, err)
			}
			entries, err := directory.ReadDir()
			if err != nil || len(entries) != 1 {
				t.Fatalf("entries=%v error=%v", entries, err)
			}
		})
	}
}
