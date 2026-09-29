package bootstrap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/agentstation/starmap/internal/privatefiles"
)

// BaselineRecoveryDirectoryName identifies the journal directory within a baseline export tree.
const BaselineRecoveryDirectoryName = baselineRecoveryDirectory

// BaselineRetainedFile identifies captured bytes in a verified baseline inventory.
type BaselineRetainedFile = privatefiles.RetainedFile

// BaselineRecordReader reads bounded metadata from a verified backup.
type BaselineRecordReader = privatefiles.RetainedRecordReader

// InspectRetainedBaselinePublications selects matching baseline stages for inactive retention.
// The inventory includes the baseline files and the separate recovery directory.
// The caller preserves selected files and every recovery record in inactive evidence.
// Completed exports require separate validation. This check never promotes staging bytes or authorizes native cleanup.
func InspectRetainedBaselinePublications(ctx context.Context, files map[string]BaselineRetainedFile, read BaselineRecordReader) ([]string, error) {
	if ctx == nil || read == nil || len(files) > baselineRecoveryEntryLimit {
		return nil, baselineInspectionError("retained baseline inspection requires bounded input")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	journals, stages, err := retainedBaselineNames(files)
	if err != nil {
		return nil, err
	}
	records := make(map[string][]baselineJournalRecord)
	remaining := int64(baselineRecoveryByteLimit)
	for _, name := range journals {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		remaining -= files[name].Size
		if remaining < 0 {
			return nil, baselineInspectionError("retained baseline journals exceed the byte limit")
		}
		record, err := readRetainedBaselineJournal(ctx, name, files[name], read)
		if err != nil {
			return nil, err
		}
		records[record.Stage] = append(records[record.Stage], record)
	}
	lock, hasLock := files[path.Join(baselineRecoveryDirectory, baselineRecoveryLock)]
	if len(journals) > 0 || hasLock {
		empty := sha256.Sum256(nil)
		if !hasLock || lock.Size != 0 || lock.SHA256 != hex.EncodeToString(empty[:]) {
			return nil, baselineInspectionError("retained baseline writer record is missing or invalid")
		}
	}
	var inactive []string
	for stage, names := range stages {
		matched := false
		for _, record := range records[stage] {
			if retainedBaselineStageMatches(stage, names, files, record) {
				matched = true
				break
			}
		}
		if !matched {
			return nil, baselineInspectionError("retained baseline stage differs from its publication evidence")
		}
		inactive = append(inactive, names...)
	}
	slices.Sort(inactive)
	return inactive, ctx.Err()
}

func retainedBaselineNames(files map[string]BaselineRetainedFile) ([]string, map[string][]string, error) {
	var journals []string
	stages := make(map[string][]string)
	namesBytes := 0
	for name, file := range files {
		namesBytes += len(name)
		if !fs.ValidPath(name) || name == "." || strings.Contains(name, "\\") || namesBytes > 1<<20 || file.Size < 0 || !baselineDigest(file.SHA256) {
			return nil, nil, baselineInspectionError("retained baseline inventory is invalid")
		}
		first, _, nested := strings.Cut(name, "/")
		if first == baselineRecoveryDirectory {
			if !nested || path.Dir(name) != baselineRecoveryDirectory {
				return nil, nil, baselineInspectionError("retained baseline recovery record has an invalid path")
			}
			if path.Base(name) != baselineRecoveryLock {
				journals = append(journals, name)
			}
		} else if strings.HasPrefix(first, baselineStagePrefix) {
			if !nested || path.Dir(name) != first {
				return nil, nil, baselineInspectionError("retained baseline stage has an invalid path")
			}
			stages[first] = append(stages[first], name)
		}
	}
	slices.Sort(journals)
	return journals, stages, nil
}

func readRetainedBaselineJournal(ctx context.Context, name string, file BaselineRetainedFile, read BaselineRecordReader) (baselineJournalRecord, error) {
	var record baselineJournalRecord
	data, err := read(ctx, name, baselineJournalLimit)
	if err != nil {
		return record, err
	}
	sum := sha256.Sum256(data)
	if len(data) > baselineJournalLimit || int64(len(data)) != file.Size || hex.EncodeToString(sum[:]) != file.SHA256 {
		return record, baselineInspectionError("retained baseline journal changed after backup verification")
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return record, baselineInspectionError("retained baseline journal is invalid")
	}
	canonical, err := json.Marshal(record)
	if err != nil || !bytes.Equal(data, append(canonical, '\n')) {
		return record, baselineInspectionError("retained baseline journal requires canonical encoding")
	}
	journalName := path.Base(name)
	if nonce, ok := strings.CutPrefix(journalName, ".record-"); ok {
		if len(nonce) != baselineJournalTokenLen || strings.Trim(nonce, "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567") != "" {
			return record, baselineInspectionError("retained baseline journal stage has an invalid name")
		}
		journalName = strings.TrimPrefix(record.Stage, baselineStagePrefix) + ".json"
	}
	if !validBaselineJournal(journalName, record) {
		return record, baselineInspectionError("retained baseline journal does not match its schema or stage")
	}
	return record, nil
}

func retainedBaselineStageMatches(stage string, names []string, files map[string]BaselineRetainedFile, record baselineJournalRecord) bool {
	for _, name := range names {
		matched := false
		for _, entry := range record.Files {
			if name == path.Join(stage, entry.Name) && files[name].Size == entry.Size && files[name].SHA256 == entry.SHA256 {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
