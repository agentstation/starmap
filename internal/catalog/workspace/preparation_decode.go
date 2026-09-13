package workspace

import (
	"path/filepath"
	"strings"
)

func (stage *workspaceStage) acceptPreparationHeader(event preparationEvent, target, lock, identity string) (int, error) {
	h := event.Header
	if h == nil || event.Handoff != nil || event.Record != nil || event.hasRelocation() || event.Tree != "" || event.Entry != nil || event.Identity != "" || h.Version < 1 || h.Version > preparationJournalVersion ||
		h.Target != target || h.Stage != stage.name || h.LockIdentity == "" || h.JournalIdentity != identity ||
		!replacementChildName(h.Stage) || !strings.HasPrefix(h.Stage, "."+filepath.Base(target)+".preparing-") || len(h.Enclosure.Entries) != 1 {
		return 0, invalidReplacement("preparation_header")
	}
	if h.LockIdentity != lock {
		return 0, writerConflict(target)
	}
	if err := h.Enclosure.validate(); err != nil {
		return 0, err
	}
	if err := validateReplacementIdentities(h.Enclosure, h.Identities); err != nil {
		return 0, err
	}
	stage.enclosure = h.Enclosure
	stage.enclosure.identities = h.Identities
	return h.Version, nil
}

func (stage *workspaceStage) acceptPreparationEvent(event preparationEvent, target string, version int) error {
	if stage.handoff != nil {
		return invalidReplacement("preparation_handoff_suffix")
	}
	if event.hasRelocation() {
		return stage.acceptPreparationRelocation(event, target, version)
	}
	if stage.relocation != nil {
		return invalidReplacement("relocation_suffix")
	}
	if event.Record != nil {
		return stage.acceptPreparationRecord(event, target, version)
	}
	if stage.record != nil {
		return invalidReplacement("preparation_record_suffix")
	}
	if event.Handoff != nil {
		return stage.acceptPreparationHandoff(event, target, version)
	}
	return stage.acceptPreparationEntry(event)
}

func (stage *workspaceStage) acceptPreparationRelocation(event preparationEvent, target string, version int) error {
	if version < 4 || event.Header != nil || event.Handoff != nil || event.Record != nil || event.Tree != "" || event.Entry != nil || event.Identity != "" || len(stage.trees) != 0 || stage.record != nil {
		return invalidReplacement("relocation_event")
	}
	if err := stage.acceptRelocation(event, target); err != nil {
		return err
	}
	return nil
}

func (stage *workspaceStage) acceptPreparationRecord(event preparationEvent, target string, version int) error {
	if version < 3 || event.Header != nil || event.Handoff != nil || event.Entry != nil || event.Tree != "" || event.Identity != "" || len(stage.trees) != 0 {
		return invalidReplacement("preparation_record")
	}
	if err := event.Record.validate(stage, target); err != nil {
		return err
	}
	stage.record = event.Record
	return nil
}

func (stage *workspaceStage) acceptPreparationHandoff(event preparationEvent, target string, version int) error {
	if version < 2 || event.Header != nil || event.Entry != nil || event.Tree != "" || event.Identity != "" {
		return invalidReplacement("preparation_handoff")
	}
	if err := event.Handoff.bind(stage, target); err != nil {
		return err
	}
	stage.handoff = event.Handoff
	return nil
}

func (stage *workspaceStage) acceptPreparationEntry(event preparationEvent) error {
	if event.Header != nil || event.Entry == nil || event.Identity == "" || len(event.Identity) > replacementIdentityMax ||
		!replacementChildName(event.Tree) || (event.Tree != "render" && event.Tree != "tree" && !strings.HasPrefix(event.Tree, ".render.verify-")) {
		return invalidReplacement("preparation_entry")
	}
	tree := stage.trees[event.Tree]
	if tree == nil {
		if len(stage.trees) >= preparationTreeMax || event.Entry.Path != "." || !event.Entry.Directory {
			return invalidReplacement("preparation_tree")
		}
		tree = &preparationTree{entries: make(map[string]treeEntry), identities: make(map[string]string)}
		stage.trees[event.Tree] = tree
	}
	entry := *event.Entry
	if old, exists := tree.entries[entry.Path]; exists {
		if tree.identities[entry.Path] != event.Identity || old.Directory != entry.Directory {
			return invalidReplacement("preparation_entry_identity")
		}
	} else {
		if err := tree.allowEntry(entry.Path); err != nil {
			return err
		}
		tree.nameBytes += len(entry.Path)
	}
	tree.entries[entry.Path], tree.identities[entry.Path] = entry, event.Identity
	return nil
}
