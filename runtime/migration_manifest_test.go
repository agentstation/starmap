package runtime

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestMigrationManifestRejectsInvalidIntent(t *testing.T) {
	t.Parallel()
	for name, change := range map[string]func(*directoryMigrationManifest){
		"schema":          func(m *directoryMigrationManifest) { m.SchemaVersion++ },
		"operation":       func(m *directoryMigrationManifest) { m.OperationID = "" },
		"identity":        func(m *directoryMigrationManifest) { m.SourceIdentity = "line\nbreak" },
		"owner":           func(m *directoryMigrationManifest) { m.Owner.Product = "unknown" },
		"owner digest":    func(m *directoryMigrationManifest) { m.SourceOwnerSHA256 = "invalid" },
		"relative root":   func(m *directoryMigrationManifest) { m.SourceDirectory = "relative" },
		"same root":       func(m *directoryMigrationManifest) { m.TargetDirectory = m.SourceDirectory },
		"nested root":     func(m *directoryMigrationManifest) { m.TargetDirectory = filepath.Join(m.SourceDirectory, "child") },
		"parent root":     func(m *directoryMigrationManifest) { m.TargetDirectory = filepath.Dir(m.SourceDirectory) },
		"empty inventory": func(m *directoryMigrationManifest) { m.Files = nil },
		"no seed":         func(m *directoryMigrationManifest) { m.Files[0].Target = "another-file" },
		"seed length":     func(m *directoryMigrationManifest) { m.Files[0].Size-- },
		"seed source":     func(m *directoryMigrationManifest) { m.Files[0].Source = "unrelated" },
		"traversal":       func(m *directoryMigrationManifest) { m.Files[0].Source = "../instance-seed" },
		"digest":          func(m *directoryMigrationManifest) { m.Files[0].SHA256 = strings.Repeat("a", 63) },
		"duplicate":       func(m *directoryMigrationManifest) { m.Files = append(m.Files, m.Files[0]) },
		"reserved child": func(m *directoryMigrationManifest) {
			file := m.Files[0]
			file.Source, file.Target = "another-file", ".owner.lock/child"
			m.Files = append(m.Files, file)
		},
		"target ancestor": func(m *directoryMigrationManifest) {
			file := m.Files[0]
			file.Source, file.Target = "another-file", "instance-seed/child"
			m.Files = append(m.Files, file)
		},
		"source ancestor": func(m *directoryMigrationManifest) {
			file := m.Files[0]
			file.Source, file.Target = "catalog-runtime", "another-file"
			m.Files = append(m.Files, file)
		},
	} {
		t.Run(name, func(t *testing.T) {
			root, manifest := migrationJournalFixture(t)
			change(&manifest)
			if journal, err := openDirectoryMigrationJournal(t.Context(), root, manifest); err == nil {
				_ = journal.Close()
				t.Fatal("invalid migration intent was accepted")
			}
		})
	}
}

func TestMigrationManifestOrderingDoesNotMutateCaller(t *testing.T) {
	t.Parallel()
	_, manifest := migrationJournalFixture(t)
	file := manifest.Files[0]
	file.Source, file.Target = "a-file", "a-file"
	manifest.Files = append(manifest.Files, file)
	before := slices.Clone(manifest.Files)
	first, err := manifest.encode()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before, manifest.Files) {
		t.Fatal("encoding changed the caller's inventory")
	}
	slices.Reverse(manifest.Files)
	second, err := manifest.encode()
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("inventory order changed canonical intent")
	}
}
