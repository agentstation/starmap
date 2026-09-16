package productpaths

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

// WorkspaceFiles describes a selected authoring tree and its adjacent recovery files.
// An empty workspace disables the tree and omits sibling artifacts. The function does not access files.
// Selectors name the host settings that select this workspace.
func WorkspaceFiles(workspace Path, selectors ...string) ([]FileEntry, error) {
	if workspace.Path != "" && (!filepath.IsAbs(workspace.Path) || strings.ContainsRune(workspace.Path, '\x00')) {
		return nil, &errors.ValidationError{Field: "workspace.path", Message: "must be empty or absolute without NUL"}
	}
	var entries []FileEntry
	availability := "available"
	if workspace.Path == "" {
		availability = "disabled"
	}
	entries = append(entries, FileEntry{ID: "workspace", Location: workspace, Kind: "tree", Availability: availability, Patterns: []string{"**"}, Creation: "Explicit catalog authoring or projection.", Recovery: "Preserve operator content and projection receipts."})
	if availability == "disabled" {
		return workspaceFilePolicies(entries, selectors)
	}
	parent := workspace
	parent.Path = filepath.Dir(parent.Path)
	name := filepath.Base(workspace.Path)
	for _, item := range []struct{ id, suffix, creation, recovery string }{
		{"workspace-receipt", ".starmap-projection.json", "Successful workspace projection.", "Preserve with the human catalog workspace."},
		{"workspace-journal", ".starmap-replacement.json", "Windows workspace replacement records intent before either directory moves.", "Preserve with the candidate and backup. Projection repair validates and resumes the operation."},
		{"workspace-lock", ".starmap-write.lock", "First writer creates the lock. Shared reads and exclusive writes reuse it.", "Retain while the workspace is in use. A lock file alone does not prove active ownership."},
	} {
		location := parent
		location.Path = filepath.Join(parent.Path, "."+name+item.suffix)
		entries = append(entries, FileEntry{ID: item.id, Location: location, Kind: "file", Availability: availability, Creation: item.creation, Recovery: item.recovery})
	}
	patternName := strings.NewReplacer("\\", "\\\\", "*", "\\*", "?", "\\?", "[", "\\[", "]", "\\]").Replace(name)
	entries = append(entries,
		FileEntry{ID: "catalog-migration-lock", Location: parent, Kind: "patterns", Availability: availability, Patterns: []string{"." + patternName + ".starmap-migration-lock-*"}, Creation: "Windows migration creates a hard link to the existing private store commit lock.", Recovery: "Completed operations remove their own alias. Stop all store writers and migrations before removing an abandoned alias."},
		FileEntry{ID: "workspace-preparing", Location: parent, Kind: "patterns", Availability: availability, Patterns: []string{"." + patternName + ".preparing-*"}, Creation: "Private workspace rendering and access restoration.", Recovery: "Preserve interrupted preparation until ownership checks permit cleanup."},
		FileEntry{ID: "workspace-staging", Location: parent, Kind: "patterns", Availability: availability, Patterns: []string{"." + patternName + ".preparing-*/**", "." + patternName + ".candidate-*/**", ".." + patternName + ".candidate-*.verify-*/**", ".." + patternName + ".starmap-projection.json.*", ".." + patternName + ".starmap-replacement.json.*"}, Creation: "Workspace copy, verification, receipt, and journal writes.", Recovery: "Preserve interrupted work until ownership and recovery checks permit cleanup."},
		FileEntry{ID: "workspace-backup", Location: parent, Kind: "patterns", Availability: availability, Patterns: []string{"." + patternName + ".backup-*/**"}, Creation: "Windows replacement retains the previous workspace until the new receipt is saved.", Recovery: "Retain with the replacement journal. Recovery refuses changed or unrecognized backup files."},
	)
	return workspaceFilePolicies(entries, selectors)
}

func workspaceFilePolicies(entries []FileEntry, selectors []string) ([]FileEntry, error) {
	for i := range entries {
		access, err := policy.ForRole(entries[i].ID)
		if err != nil {
			return nil, err
		}
		entries[i].Policy = FilePolicy{
			Selectors:     slices.Clone(selectors),
			Applicability: "Only the selected workspace and explicit authoring operations use these files.",
			Access:        access,
			Retention:     "Preserve operator input and recovery evidence until verified recovery permits removal.",
			Removal:       "This inventory does not authorize deletion.",
		}
	}
	return entries, nil
}
