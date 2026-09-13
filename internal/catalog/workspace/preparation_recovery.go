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

func recoverPreparationsExcept(ctx context.Context, target string, writer *workspaceWriter, retained *workspaceStage) error {
	if err := writer.check(); err != nil {
		return err
	}
	if writer.target != target {
		return writerConflict(target)
	}
	names, err := preparationNames(ctx, target)
	if err != nil {
		return err
	}
	for _, name := range names {
		if retained != nil && name == retained.name {
			if err := retained.journal.unchanged(ctx); err != nil {
				return err
			}
			continue
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

func preparationNames(ctx context.Context, target string) ([]string, error) {
	parent, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = parent.Close() }()
	file, err := parent.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	var names []string
	seen := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, err := file.ReadDir(replacementReadBatch)
		seen += len(entries)
		if seen > preparationScanMax {
			return nil, replacementLimit("preparation_scan")
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
			return nil, err
		}
	}
	slices.Sort(names)
	return names, nil
}

func readPreparation(ctx context.Context, target, name string, writer *workspaceWriter) (_ *workspaceStage, resultErr error) {
	if err := writer.check(); err != nil {
		return nil, err
	}
	if writer.target != target {
		return nil, writerConflict(target)
	}
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
	version := 0
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
			var err error
			version, err = stage.acceptPreparationHeader(event, target, lock, identity)
			if err != nil {
				return err
			}
			continue
		}
		if err := stage.acceptPreparationEvent(event, target, version); err != nil {
			return err
		}
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
