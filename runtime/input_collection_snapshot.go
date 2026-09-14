package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/agentstation/starmap/internal/privatefiles"
)

// inputCollectionSnapshot owns the bounded raw records used by one collection.
type inputCollectionSnapshot struct {
	request     InputCollectionRequest
	report      InputCollectionReport
	directory   *privatefiles.Directory
	manual      []byte
	publication []byte
	records     map[string][]byte
}

func (s *inputCollectionSnapshot) capture(ctx context.Context, store *layerStore) error {
	var err error
	s.manual, err = s.readRoot(store.directory, manualHistoryName)
	if err != nil {
		return err
	}
	s.publication, err = s.readRoot(store.directory, inputPublicationName)
	if err != nil {
		return err
	}
	s.directory, err = store.directory.ExistingChild(inputPublicationDirectory)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	root, err := s.directory.Open()
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	listing, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { _ = listing.Close() }()
	entries, err := listing.ReadDir(s.request.MaxEntries + 1)
	if err != nil && err != io.EOF {
		return err
	}
	s.report.Scanned = len(entries)
	if len(entries) > s.request.MaxEntries {
		return invalidInputPublication("input collection exceeds the entry limit")
	}
	slices.SortFunc(entries, func(a, b os.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := entry.Name()
		if !validInputReference(name) || !entry.Type().IsRegular() {
			s.report.Preserved = append(s.report.Preserved, name)
			continue
		}
		info, err := root.Lstat(name)
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxLayerBytes {
			s.report.Preserved = append(s.report.Preserved, name)
			continue
		}
		remaining := s.request.MaxBytes - s.report.SnapshotBytes
		if info.Size() > remaining {
			return invalidInputPublication("input collection exceeds the snapshot byte limit")
		}
		raw, err := s.directory.ReadFile(name, min(remaining, int64(maxLayerBytes)))
		if err != nil {
			s.report.Preserved = append(s.report.Preserved, name)
			continue
		}
		s.report.SnapshotBytes += int64(len(raw))
		digest := sha256.Sum256(raw)
		if name != hex.EncodeToString(digest[:])+".json" {
			s.report.Preserved = append(s.report.Preserved, name)
			continue
		}
		s.records[name] = raw
	}
	return ctx.Err()
}

func (s *inputCollectionSnapshot) readRoot(directory *privatefiles.Directory, name string) ([]byte, error) {
	raw, err := directory.ReadFile(name, min(s.request.MaxBytes-s.report.SnapshotBytes, int64(maxLayerBytes)))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.report.SnapshotBytes += int64(len(raw))
	return raw, nil
}

func (s *inputCollectionSnapshot) checkRoots(store *layerStore) error {
	for _, item := range []struct {
		name string
		raw  []byte
	}{{manualHistoryName, s.manual}, {inputPublicationName, s.publication}} {
		raw, err := readLayerFile(store.directory, item.name)
		if err != nil {
			return err
		}
		if !bytes.Equal(raw, item.raw) || (raw == nil) != (item.raw == nil) {
			return invalidInputPublication("retained input references changed during collection")
		}
	}
	return nil
}
