package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/internal/filepublish"
	"github.com/agentstation/starmap/pkg/catalogs"
)

type relocationTree struct {
	Tree       treeSnapshot      `json:"tree"`
	Identities map[string]string `json:"identities"`
}

func (t *relocationTree) bind() error {
	if err := t.Tree.validate(); err != nil {
		return err
	}
	if err := validateReplacementIdentities(t.Tree, t.Identities); err != nil {
		return err
	}
	t.Tree.identities = t.Identities
	return nil
}

type relocationFile struct {
	Entry    treeEntry `json:"entry"`
	Identity string    `json:"identity"`
}

func (f relocationFile) state() workspaceRecordState {
	return workspaceRecordState{entry: f.Entry, identity: f.Identity}
}

type relocationRecord struct {
	State          string          `json:"state"`
	OriginalParent string          `json:"original_parent"`
	StateParent    string          `json:"state_parent"`
	Root           relocationTree  `json:"root"`
	Current        string          `json:"current"`
	Retained       int             `json:"retained"`
	Alias          *relocationFile `json:"alias,omitempty"`
}

type relocationGeneration struct {
	Name     string         `json:"name"`
	Snapshot relocationTree `json:"snapshot"`
}

type relocationInventory struct {
	record      relocationRecord
	generations map[string]treeSnapshot
	ready       bool
	workspace   treeSnapshot
}

func (e preparationEvent) hasRelocation() bool {
	return e.Relocation != nil || e.Generation != nil || e.RelocationReady || e.Workspace != nil
}

func (s *workspaceStage) acceptRelocation(e preparationEvent, target string) error {
	count := 0
	for _, present := range []bool{e.Relocation != nil, e.Generation != nil, e.RelocationReady, e.Workspace != nil} {
		if present {
			count++
		}
	}
	if count != 1 {
		return invalidReplacement("relocation_event")
	}
	if e.Relocation != nil {
		if s.relocation != nil {
			return invalidReplacement("relocation_header")
		}
		r := *e.Relocation
		if !filepath.IsAbs(r.State) || filepath.Clean(r.State) != r.State || !replacementDigest(r.Current) || r.Retained <= 0 || r.Retained >= preparationEventMax {
			return invalidReplacement("relocation_header")
		}
		if err := ValidateMachineSeparation(target, r.State, "catalog state"); err != nil {
			return err
		}
		for _, id := range []string{r.OriginalParent, r.StateParent} {
			if id == "" || len(id) > replacementIdentityMax {
				return invalidReplacement("relocation_parent")
			}
		}
		if err := r.Root.bind(); err != nil {
			return err
		}
		entries := r.Root.Tree.Entries
		if len(entries) != 4 || entries[1].Path != ".commit.lock" || entries[1].Directory || entries[2].Path != "current" || entries[2].Directory || entries[3].Path != "generations" || !entries[3].Directory {
			return invalidReplacement("relocation_root")
		}
		if r.Alias != nil {
			f := r.Alias
			prefix := "." + filepath.Base(target) + ".starmap-migration-lock-"
			if !replacementChildName(f.Entry.Path) || !strings.HasPrefix(f.Entry.Path, prefix) || len(f.Entry.Path) == len(prefix) || f.Identity != r.Root.Identities[".commit.lock"] {
				return invalidReplacement("relocation_alias")
			}
			expected := entries[1]
			expected.Path = f.Entry.Path
			if expected != f.Entry {
				return invalidReplacement("relocation_alias")
			}
		}
		s.relocation = &relocationInventory{record: r, generations: make(map[string]treeSnapshot)}
		return nil
	}
	r := s.relocation
	if r == nil {
		return invalidReplacement("relocation_header")
	}
	if e.Generation != nil {
		g := e.Generation
		if r.ready || !replacementDigest(g.Name) || len(r.generations) >= r.record.Retained {
			return invalidReplacement("relocation_generation")
		}
		if _, exists := r.generations[g.Name]; exists {
			return invalidReplacement("relocation_generation_duplicate")
		}
		if err := g.Snapshot.bind(); err != nil {
			return err
		}
		r.generations[g.Name] = g.Snapshot.Tree
		return nil
	}
	if e.RelocationReady {
		if r.ready || len(r.generations) != r.record.Retained {
			return invalidReplacement("relocation_inventory")
		}
		r.ready = true
		return nil
	}
	if !r.ready || r.workspace.ID != "" {
		return invalidReplacement("relocation_workspace")
	}
	if err := e.Workspace.bind(); err != nil {
		return err
	}
	r.workspace = e.Workspace.Tree
	return nil
}

func (s *workspaceStage) appendRelocation(ctx context.Context, e preparationEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.journal.check(); err != nil {
		return err
	}
	if err := s.acceptRelocation(e, s.journal.writer.target); err != nil {
		return err
	}
	return s.journal.append(e)
}

func (s *workspaceStage) recordRelocationWorkspace(ctx context.Context, tree treeSnapshot) error {
	return s.appendRelocation(ctx, preparationEvent{Workspace: &relocationTree{Tree: tree, Identities: tree.identities}})
}

func (s *workspaceStage) retireRelocation(ctx context.Context) error {
	if err := s.checkChildren(ctx); err != nil {
		return err
	}
	s.relocation = nil
	return s.close(ctx)
}

func relocationCurrent(g catalogs.Generation) (string, error) {
	data, err := json.Marshal(g.Manifest)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func relocationParent(root *os.Root) (string, error) {
	file, err := openStagedDirectory(root, ".")
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	return filepublish.Identity(file)
}

func relocationRoot(ctx context.Context, path string) (treeSnapshot, error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return treeSnapshot{}, err
	}
	defer func() { _ = root.Close() }()
	tree, err := trackPreparationTree(root)
	if err != nil {
		return treeSnapshot{}, err
	}
	scanner := treeScanner{ctx: ctx, root: root, identities: tree.identities}
	for _, name := range []string{".commit.lock", "current", "generations"} {
		entry, err := scanner.entry(name)
		if err != nil {
			return treeSnapshot{}, err
		}
		tree.entries[name] = entry
	}
	return tree.snapshot()
}
