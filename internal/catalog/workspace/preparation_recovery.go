package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	stderrors "errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/agentstation/starmap/internal/privatefiles"
)

func recoverPreparations(ctx context.Context, target string, writer *workspaceWriter) error {
	if err := writer.check(); err != nil {
		return err
	}
	if writer.target != target {
		return writerConflict(target)
	}
	parent, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		return err
	}
	defer func() { _ = parent.Close() }()
	file, err := parent.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	var names []string
	seen := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, err := file.ReadDir(replacementReadBatch)
		seen += len(entries)
		if seen > preparationScanMax {
			return replacementLimit("preparation_scan")
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "."+filepath.Base(target)+".preparing-") {
				names = append(names, entry.Name())
			}
		}
		if stderrors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
	}
	slices.Sort(names)
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return err
		}
		stage, err := readPreparation(ctx, target, name, writer)
		if err != nil {
			return err
		}
		if err := stage.close(ctx); err != nil {
			return err
		}
	}
	return nil
}

func readPreparation(ctx context.Context, target, name string, writer *workspaceWriter) (_ *workspaceStage, resultErr error) {
	parent, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		return nil, err
	}
	s := &workspaceStage{parent: parent, name: name, trees: make(map[string]*preparationTree)}
	defer func() {
		if resultErr != nil {
			if s.journal != nil {
				_ = s.journal.file.Close()
			}
			if s.private != nil {
				_ = s.private.Close()
			}
			_ = parent.Close()
		}
	}()
	info, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, invalidReplacement("preparation_directory")
	}
	if err := privatefiles.ValidateMetadata(info, "workspace preparation"); err != nil {
		return nil, err
	}
	if err := privatefiles.ValidateACL(parent, name, info, "workspace preparation"); err != nil {
		return nil, err
	}
	s.private, err = parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	data, state, err := readWorkspaceRecord(s.private, preparationJournalName, preparationJournalMax)
	if err != nil {
		return nil, err
	}
	if err := decodePreparation(ctx, s, target, writer.identity, data, state.identity); err != nil {
		return nil, err
	}
	info, err = s.private.Lstat(preparationJournalName)
	if err != nil {
		return nil, err
	}
	file, err := openSnapshotEntry(s.private, preparationJournalName, info)
	if err != nil {
		return nil, err
	}
	s.journal = &preparationJournal{stage: s, writer: writer, file: file, state: state, hash: sha256.New()}
	_, _ = s.journal.hash.Write(data)
	if err := s.journal.unchanged(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func decodePreparation(ctx context.Context, stage *workspaceStage, target, lock string, data []byte, identity string) error {
	if len(data) == 0 || len(data) > preparationJournalMax || data[len(data)-1] != '\n' {
		return invalidReplacement("preparation_journal")
	}
	if bytes.Count(data, []byte{'\n'}) > preparationEventMax {
		return replacementLimit("preparation_events")
	}
	events := bytes.Split(data[:len(data)-1], []byte{'\n'})
	for i, line := range events {
		if err := ctx.Err(); err != nil {
			return err
		}
		var event preparationEvent
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&event); err != nil {
			return invalidReplacement("preparation_event")
		}
		if err := decoder.Decode(new(any)); !stderrors.Is(err, io.EOF) {
			return invalidReplacement("preparation_event_suffix")
		}
		if i == 0 {
			h := event.Header
			if h == nil || event.Tree != "" || event.Entry != nil || event.Identity != "" || h.Version != preparationJournalVersion ||
				h.Target != target || h.Stage != stage.name || h.LockIdentity != lock || h.JournalIdentity != identity ||
				!replacementChildName(h.Stage) || !strings.HasPrefix(h.Stage, "."+filepath.Base(target)+".preparing-") || len(h.Enclosure.Entries) != 1 {
				return invalidReplacement("preparation_header")
			}
			if err := h.Enclosure.validate(); err != nil {
				return err
			}
			if err := validateReplacementIdentities(h.Enclosure, h.Identities); err != nil {
				return err
			}
			stage.enclosure = h.Enclosure
			stage.enclosure.identities = h.Identities
			continue
		}
		if event.Header != nil || event.Entry == nil || event.Identity == "" || len(event.Identity) > replacementIdentityMax ||
			!replacementChildName(event.Tree) || !(event.Tree == "render" || event.Tree == "tree" || strings.HasPrefix(event.Tree, ".render.verify-")) {
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
	}
	for _, tree := range stage.trees {
		snapshot, err := tree.snapshot()
		if err != nil {
			return err
		}
		if err := snapshot.validate(); err != nil {
			return err
		}
	}
	return nil
}
