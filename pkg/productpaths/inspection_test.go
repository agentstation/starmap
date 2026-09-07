package productpaths

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInspectionReadsMatchingMetadataWithoutFollowingLinks(t *testing.T) {
	root := t.TempDir()
	for name, contents := range map[string]string{"generations/one/catalog.json": "private-catalog-sentinel", "generations/one/manifest.json": "manifest", "unrelated/input.txt": "unrelated"} {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "catalog.json"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "generations", "linked")); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	report, err := InspectManifest(t.Context(), FileManifest{Files: []FileEntry{{ID: "catalog", Availability: "available", Location: Path{Path: root}, Patterns: []string{"generations/*/catalog.json", "generations/*/manifest.json"}}}}, 100)
	if err != nil || !report.Complete {
		t.Fatalf("incomplete metadata inspection: %+v %v", report, err)
	}
	seen := make(map[string]FileObservation)
	for _, item := range report.Observations {
		seen[item.Path] = item
		if strings.Contains(item.Path, "unrelated") || strings.HasPrefix(item.Path, filepath.Join(root, "generations", "linked")+string(filepath.Separator)) {
			t.Fatalf("inspection traversed unrelated or linked content: %+v", item)
		}
	}
	file := filepath.Join(root, "generations", "one", "catalog.json")
	if seen[filepath.Join(root, "generations", "linked")].Kind != "symlink" {
		t.Fatal("inspection hides a symbolic link that blocks a managed tree")
	}
	item := seen[file]
	if item.State != "present" || item.Kind != "file" {
		t.Fatalf("managed payload metadata missing: %+v", item)
	}
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		if item.Mode != "0600" || item.Owner == nil || !item.Owner.MatchesEffectiveUser || item.PermissionScope != "posix-mode-bits-only" {
			t.Fatalf("POSIX metadata missing: %+v", item)
		}
	} else if runtime.GOOS == "windows" {
		if item.Mode != "" || item.Owner != nil || item.WindowsSecurity == nil || item.WindowsSecurity.OwnerSID == "" || item.PermissionScope != "windows-owner-and-dacl" {
			t.Fatalf("Windows security metadata missing: %+v", item)
		}
	} else if item.Mode != "" || item.Owner != nil || item.PermissionScope != "native-permissions-unverified" {
		t.Fatal("unsupported permission metadata claims qualification")
	}
	contents, err := os.ReadFile(file)
	if err != nil || string(contents) != "private-catalog-sentinel" {
		t.Fatal("inspection changed payload bytes")
	}
}

func TestInspectionBudgetCountsUnmatchedEntries(t *testing.T) {
	root := t.TempDir()
	for i := range 50 {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("unmatched-%02d", i)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	manifest := FileManifest{Files: []FileEntry{{ID: "bounded", Availability: "available", Location: Path{Path: root}, Patterns: []string{"*.json"}}}}
	report, err := InspectManifest(t.Context(), manifest, 8)
	if err != nil || report.Complete || report.Examined != 8 || len(report.Observations) != 1 {
		t.Fatalf("directory scan exceeded its budget or claimed complete: %+v %v", report, err)
	}
	complete, err := InspectManifest(t.Context(), manifest, 100)
	if err != nil || !complete.Complete || complete.Examined != 51 {
		t.Fatalf("complete scan lost unmatched entries: %+v %v", complete, err)
	}
}

func TestInspectionReportsAbsentInapplicableAndUnavailable(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := FileManifest{Files: []FileEntry{
		{ID: "missing", Availability: "available", Location: Path{Path: filepath.Join(root, "missing")}},
		{ID: "disabled", Availability: "disabled"},
		{ID: "planned", Availability: "planned", Location: Path{Path: filepath.Join(file, "planned")}},
		{ID: "unavailable", Availability: "available", Location: Path{Path: filepath.Join(file, "child")}},
	}}
	report, err := InspectManifest(t.Context(), manifest, 100)
	if err != nil || report.Complete || len(report.Observations) != 4 {
		t.Fatalf("invalid inspection coverage: %+v %v", report, err)
	}
	for i, want := range []string{"absent", "not-applicable", "not-applicable", "unavailable"} {
		if report.Observations[i].State != want {
			t.Fatalf("observation %d: %+v", i, report.Observations[i])
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatal("inspection created reserved or missing paths")
	}
}

func TestInspectionValidatesBeforeMetadataAccess(t *testing.T) {
	for _, test := range []struct {
		name     string
		manifest FileManifest
		limit    int
	}{
		{"negative-limit", FileManifest{}, -1},
		{"oversized-limit", FileManifest{}, MaximumInspectionEntries + 1},
		{"relative-path", FileManifest{Files: []FileEntry{{ID: "one", Availability: "available", Location: Path{Path: "relative"}}}}, 100},
		{"invalid-glob", FileManifest{Files: []FileEntry{{ID: "one", Availability: "available", Location: Path{Path: t.TempDir()}, Patterns: []string{"["}}}}, 100},
		{"traversal-pattern", FileManifest{Files: []FileEntry{{ID: "one", Availability: "available", Location: Path{Path: t.TempDir()}, Patterns: []string{"../*"}}}}, 100},
		{"nonterminal-double-star", FileManifest{Files: []FileEntry{{ID: "one", Availability: "available", Location: Path{Path: t.TempDir()}, Patterns: []string{"**/file"}}}}, 100},
	} {
		t.Run(test.name, func(t *testing.T) {
			report, err := InspectManifest(t.Context(), test.manifest, test.limit)
			if err == nil || report.Examined != 0 {
				t.Fatal("invalid inspection input reached metadata access")
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	report, err := InspectManifest(ctx, FileManifest{Files: []FileEntry{{ID: "one", Availability: "available", Location: Path{Path: t.TempDir()}}}}, 100)
	if !errors.Is(err, context.Canceled) || report.Examined != 0 || report.Complete {
		t.Fatal("cancelled inspection read files or claimed complete")
	}
}

func TestInspectionEscapedNamesAndRecursivePatterns(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, ".work[1].candidate-a", "nested", "payload")
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := InspectManifest(t.Context(), FileManifest{Files: []FileEntry{{ID: "stage", Availability: "available", Location: Path{Path: root}, Patterns: []string{`.work\[1\].candidate-*/**`}}}}, 100)
	if err != nil || !report.Complete {
		t.Fatalf("escaped scan failed: %+v %v", report, err)
	}
	found := false
	for _, item := range report.Observations {
		if item.Path == file && item.Kind == "file" {
			found = true
		}
	}
	if !found {
		t.Fatal("recursive escaped pattern omitted the payload")
	}
}
