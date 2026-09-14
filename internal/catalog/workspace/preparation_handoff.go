package workspace

import (
	"context"
	"path/filepath"
	"strings"
)

type preparationHandoff struct {
	Candidate          string            `json:"candidate"`
	NewIdentity        string            `json:"new_identity"`
	Original           treeSnapshot      `json:"original"`
	OriginalIdentities map[string]string `json:"original_identities"`
	prepared           treeSnapshot
}

type preparationOwner struct {
	target  string
	name    string
	writer  *workspaceWriter
	journal workspaceRecordState
}

func (s *workspaceStage) recordHandoff(prepared treeSnapshot) error {
	h := &preparationHandoff{
		Candidate: s.candidate, NewIdentity: prepared.ID,
		Original: s.original, OriginalIdentities: s.original.identities,
	}
	if err := h.bind(s, s.journal.writer.target); err != nil {
		return err
	}
	if err := s.journal.append(preparationEvent{Handoff: h}); err != nil {
		return err
	}
	s.handoff = h
	return nil
}

func (h *preparationHandoff) bind(stage *workspaceStage, target string) error {
	prefix := "." + filepath.Base(target) + ".preparing-"
	suffix, found := strings.CutPrefix(stage.name, prefix)
	if !found || suffix == "" || h.Candidate != "."+filepath.Base(target)+".candidate-"+suffix || !replacementChildName(h.Candidate) {
		return invalidReplacement("preparation_candidate")
	}
	tree := stage.trees["tree"]
	if tree == nil {
		return invalidReplacement("preparation_candidate_tree")
	}
	prepared, err := tree.snapshot()
	if err != nil {
		return err
	}
	if err := prepared.validate(); err != nil {
		return err
	}
	if prepared.ID != h.NewIdentity || h.Original.ID == h.NewIdentity {
		return invalidReplacement("preparation_candidate_identity")
	}
	if h.Original.ID != "" {
		if err := h.Original.validate(); err != nil {
			return err
		}
		if err := validateReplacementIdentities(h.Original, h.OriginalIdentities); err != nil {
			return err
		}
	} else if h.Original.Digest != "" || len(h.Original.Entries) != 0 || len(h.OriginalIdentities) != 0 {
		return invalidReplacement("preparation_original")
	}
	h.Original.identities = h.OriginalIdentities
	h.prepared = prepared
	return nil
}

func (s *workspaceStage) cleanupCandidate(ctx context.Context) error {
	if s.handoff == nil {
		return nil
	}
	if err := pendingReplacement(s.journal.writer.target); err != nil {
		return err
	}
	return cleanupWorkspaceTreeAtChecked(ctx, s.parent, s.handoff.Candidate, s.checkWriter, s.handoff.prepared, s.handoff.Original)
}

func (o *preparationOwner) cleanup(ctx context.Context) error {
	stage, err := o.read(ctx)
	if err != nil {
		return err
	}
	return stage.close(ctx)
}

func (o *preparationOwner) read(ctx context.Context) (*workspaceStage, error) {
	if err := o.writer.check(); err != nil {
		return nil, err
	}
	stage, err := readPreparation(ctx, o.target, o.name, o.writer)
	if err != nil {
		return nil, err
	}
	if stage.handoff == nil || stage.journal.state != o.journal {
		stage.releaseHandles()
		return nil, replacementConflict(o.name, "candidate preparation journal changed after handoff")
	}
	return stage, nil
}

func (s stagedWorkspace) validatePublication(ctx context.Context) error {
	var stage *workspaceStage
	if s.owner != nil {
		var err error
		stage, err = s.owner.read(ctx)
		if err != nil {
			return err
		}
		defer stage.releaseHandles()
		if s.path != filepath.Join(filepath.Dir(s.owner.target), stage.handoff.Candidate) || !sameReplacementTree(s.tree, stage.handoff.prepared) {
			return invalidReplacement("preparation_publication")
		}
		if err := stage.checkChildren(ctx); err != nil {
			return err
		}
	}
	actual, err := snapshotTree(ctx, s.path)
	if err != nil {
		return err
	}
	if !sameReplacementTree(actual, s.tree) {
		return replacementConflict(s.path, "candidate changed before publication")
	}
	if stage != nil {
		return stage.checkWriter()
	}
	return nil
}
