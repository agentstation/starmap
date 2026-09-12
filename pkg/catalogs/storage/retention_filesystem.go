package storage

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

type retainedFilesystemEntry struct {
	entry   retentionEntry
	receipt retirementJournal
}

// Collect removes obsolete generations through checked, recoverable retirement.
// The publication lock serializes each pass with publishers and ordinary reads.
// Unknown entries and changed receipts stop cleanup without deleting those entries.
func (s *Filesystem) Collect(ctx context.Context, request RetentionRequest) (RetentionReport, error) {
	if err := ctx.Err(); err != nil {
		return RetentionReport{}, err
	}
	limit, err := request.scanLimit()
	if err != nil {
		return RetentionReport{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Lstat(s.root); os.IsNotExist(err) && request.ExpectedGenerationID == "" && len(request.RequiredGenerationIDs) == 0 {
		return RetentionReport{}, nil
	}
	unlock, err := s.lockRetention(ctx)
	if err != nil {
		return RetentionReport{}, err
	}
	defer func() { _ = unlock() }()
	current, err := s.currentIDOrEmpty()
	if err != nil {
		return RetentionReport{}, err
	}
	if current != request.ExpectedGenerationID {
		return RetentionReport{}, casConflict(request.ExpectedGenerationID, current)
	}
	required := make(map[string]bool, len(request.RequiredGenerationIDs)+1)
	required[current] = true
	for _, id := range request.RequiredGenerationIDs {
		required[id] = true
		if _, err := s.readGeneration(ctx, id); err != nil {
			return RetentionReport{}, err
		}
	}
	directory, err := privatefiles.ExistingDirectory(filepath.Join(s.root, "generations"))
	if err != nil {
		return RetentionReport{}, err
	}
	parent, err := directory.Open()
	if err != nil {
		return RetentionReport{}, err
	}
	defer func() { _ = parent.Close() }()
	if _, err := readRetentionEntries(parent, limit); err != nil {
		return RetentionReport{}, err
	}
	if request.DryRun {
		err = directory.CheckNoPendingPublications(ctx)
	} else {
		err = directory.RecoverPublications(ctx)
	}
	if err != nil {
		return RetentionReport{}, err
	}
	names, err := readRetentionEntries(parent, limit)
	if err != nil {
		return RetentionReport{}, err
	}
	if err := s.recoverRetirements(ctx, parent, names, required, request.DryRun); err != nil {
		return RetentionReport{}, err
	}
	names, err = readRetentionEntries(parent, limit)
	if err != nil {
		return RetentionReport{}, err
	}
	entries, stored, err := s.scanRetainedGenerations(ctx, parent, names, required)
	if err != nil {
		return RetentionReport{}, err
	}
	if current != "" {
		if _, found := stored[current]; !found {
			return RetentionReport{}, generationNotFound(current)
		}
	}
	report, err := selectRetention(ctx, request, entries)
	if err != nil || request.DryRun {
		return report, err
	}
	for _, id := range report.Candidates {
		entry := stored[id]
		if err := s.retireGeneration(ctx, parent, entry.receipt); err != nil {
			return report, err
		}
		report.Removed = append(report.Removed, id)
		report.After.Generations--
		report.After.Bytes -= entry.entry.bytes
	}
	return report, nil
}

func (s *Filesystem) scanRetainedGenerations(ctx context.Context, parent *os.Root, names []fs.DirEntry, required map[string]bool) ([]retentionEntry, map[string]retainedFilesystemEntry, error) {
	entries := make([]retentionEntry, 0, len(names))
	stored := make(map[string]retainedFilesystemEntry, len(names))
	for _, name := range names {
		if name.Name() == privatefiles.PublicationDirectoryName {
			continue
		}
		if strings.HasPrefix(name.Name(), retirementPrefix) || strings.HasPrefix(name.Name(), retiredPrefix) {
			return nil, nil, retentionConflict(name.Name(), "retirement recovery requires an explicit collection pass")
		}
		entry, err := s.captureRetention(ctx, parent, name.Name())
		if err != nil {
			return nil, nil, err
		}
		id := entry.entry.id
		lock, available, err := s.lockGeneration(id, false)
		if err != nil {
			return nil, nil, err
		}
		if lock != nil {
			if err := lock.Close(); err != nil {
				return nil, nil, err
			}
		}
		entry.entry.protected = required[id] || !available
		stored[id] = entry
		entries = append(entries, entry.entry)
	}
	return entries, stored, nil
}

func (s *Filesystem) captureRetention(ctx context.Context, parent *os.Root, name string) (retainedFilesystemEntry, error) {
	if err := ctx.Err(); err != nil {
		return retainedFilesystemEntry{}, err
	}
	if !generationDirectoryName(name) {
		return retainedFilesystemEntry{}, retentionConflict(name, "unrecognized generation entry")
	}
	identity, err := privatefiles.CaptureEntry(parent, name, true)
	if err != nil {
		return retainedFilesystemEntry{}, err
	}
	root, err := parent.OpenRoot(name)
	if err != nil {
		return retainedFilesystemEntry{}, err
	}
	defer func() { _ = root.Close() }()
	data, err := privatefiles.ReadFile(root, manifestFilename, MaxFilesystemManifestBytes)
	if err != nil {
		return retainedFilesystemEntry{}, err
	}
	manifest, err := parseRetentionManifest(data, name)
	if err != nil {
		return retainedFilesystemEntry{}, err
	}
	if err := validateFilesystemRecordSize(payloadFilename, manifest.Payload.SizeBytes); err != nil {
		return retainedFilesystemEntry{}, err
	}
	payload, err := privatefiles.ReadFile(root, payloadFilename, manifest.Payload.SizeBytes)
	if err != nil {
		return retainedFilesystemEntry{}, err
	}
	generation := catalogs.Generation{Manifest: manifest, Payload: payload}
	if err := generation.Validate(); err != nil {
		return retainedFilesystemEntry{}, err
	}
	authority, err := authorityRecordData(generation)
	if err != nil {
		return retainedFilesystemEntry{}, err
	}
	names, err := readRetentionEntries(root, maxRetirementRecords)
	if err != nil {
		return retainedFilesystemEntry{}, err
	}
	receipt := retirementJournal{Version: retirementVersion, GenerationID: manifest.GenerationID, Source: name, Directory: identity}
	for _, entry := range names {
		limit, err := retentionRecordLimit(entry.Name())
		if err != nil {
			return retainedFilesystemEntry{}, err
		}
		file, err := privatefiles.CaptureRecord(root, entry.Name(), limit)
		if err != nil {
			return retainedFilesystemEntry{}, err
		}
		if (entry.Name() == manifestFilename && file.Digest != retentionDigest(data)) ||
			(entry.Name() == payloadFilename && file.Digest != retentionDigest(payload)) {
			return retainedFilesystemEntry{}, retentionConflict(name, "generation changed during the retention scan")
		}
		if entry.Name() == authorityFilename && (authority == nil || file.Digest != retentionDigest(authority)) {
			return retainedFilesystemEntry{}, retentionConflict(name, "authority record does not match the generation")
		}
		receipt.Files = append(receipt.Files, retirementRecord{Name: entry.Name(), Receipt: file})
	}
	if err := privatefiles.CheckEntry(parent, name, true, identity); err != nil {
		return retainedFilesystemEntry{}, err
	}
	return retainedFilesystemEntry{entry: retentionEntry{id: manifest.GenerationID, generatedAt: generation.Manifest.GeneratedAt,
		bytes: int64(len(data)) + int64(len(generation.Payload))}, receipt: receipt}, nil
}

func readRetentionEntries(root *os.Root, limit int) ([]fs.DirEntry, error) {
	file, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	entries, err := file.ReadDir(limit + 1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(entries) > limit {
		return nil, retentionConflict(root.Name(), "entry count exceeds the scan limit")
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, nil
}

func retentionRecordLimit(name string) (int64, error) {
	if name == generationReadLock {
		return 0, nil
	}
	if name != manifestFilename && name != payloadFilename && name != authorityFilename {
		return 0, retentionConflict(name, "unrecognized generation contents")
	}
	return filesystemRecordLimit(name)
}

func retentionConflict(name, message string) error {
	return &errors.ConflictError{Resource: "catalog retention", Actual: name, Message: message}
}

var _ RetainingStore = (*Filesystem)(nil)
