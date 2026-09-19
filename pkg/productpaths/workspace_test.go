package productpaths

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

func TestWorkspaceFilesSharePoliciesWithoutFilesystemAccess(t *testing.T) {
	root := t.TempDir()
	selected := Path{Path: filepath.Join(root, "workspace[local]"), Origin: "host", Anchor: root}
	selectors := []string{"HOST_WORKSPACE"}
	entries, err := WorkspaceFiles(selected, selectors...)
	if err != nil || len(entries) != 8 {
		t.Fatalf("workspace inventory: %d entries, %v", len(entries), err)
	}
	for _, entry := range entries {
		access, err := policy.ForRole(entry.ID)
		if err != nil || entry.Policy.Access != access || entry.Location.Origin != selected.Origin || entry.Location.Anchor != root {
			t.Fatalf("workspace policy or provenance differs: %+v, %v", entry, err)
		}
	}
	entries[0].Policy.Selectors[0] = "changed"
	if selectors[0] != "HOST_WORKSPACE" || entries[1].Policy.Selectors[0] != "HOST_WORKSPACE" {
		t.Fatal("workspace entries share mutable selectors")
	}
	children, err := os.ReadDir(root)
	if err != nil || len(children) != 0 {
		t.Fatalf("inventory changed the filesystem: %v", err)
	}
}

func TestWorkspaceFilesInspectOnlySelectedRecoveryArtifacts(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		".workspace[local].starmap-replacement.json", ".workspace[local].backup-123/notes.txt",
		".workspace[local].preparing-123/render/providers/openai.yaml", ".workspace-other.backup-123/notes.txt",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("private-workspace-content"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := WorkspaceFiles(Path{Path: filepath.Join(root, "workspace[local]")})
	if err != nil {
		t.Fatal(err)
	}
	inspection, err := InspectManifest(t.Context(), FileManifest{Files: entries}, 100)
	if err != nil || !inspection.Complete {
		t.Fatalf("workspace inspection: %+v %v", inspection, err)
	}
	seen := make(map[string]bool)
	for _, item := range inspection.Observations {
		if strings.Contains(item.Path, "workspace-other") {
			t.Fatal("inspection included another workspace")
		}
		if item.State == "present" {
			seen[item.Path] = true
		}
	}
	for _, name := range []string{".workspace[local].starmap-replacement.json", ".workspace[local].backup-123/notes.txt", ".workspace[local].preparing-123/render/providers/openai.yaml"} {
		if !seen[filepath.Join(root, filepath.FromSlash(name))] {
			t.Errorf("inspection omitted recovery artifact %s", name)
		}
	}
	encoded, err := json.Marshal(inspection)
	if err != nil || strings.Contains(string(encoded), "private-workspace-content") {
		t.Fatalf("inspection exposed contents: %v", err)
	}
}

func TestWorkspaceFilesDisabledAndInvalidSelections(t *testing.T) {
	entries, err := WorkspaceFiles(Path{})
	if err != nil || len(entries) != 1 || entries[0].Availability != "disabled" || entries[0].Location.Path != "" {
		t.Fatalf("disabled workspace: %+v %v", entries, err)
	}
	for _, path := range []string{"relative", filepath.Join(t.TempDir(), "invalid\x00path")} {
		if _, err := WorkspaceFiles(Path{Path: path}); err == nil {
			t.Errorf("accepted invalid workspace %q", path)
		}
	}
}
