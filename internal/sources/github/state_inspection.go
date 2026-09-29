package github

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"io"
	"strings"
	"unicode"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/errors"
)

const stateInspectionEntryLimit = 10000

// MaxStateRecordBytes bounds one retained discovery record.
const MaxStateRecordBytes = maxStateBytes

// InspectStateDirectory validates retained discovery records without fetching or writing.
// The caller must fence writers and verify complete inventory around this check.
// Validation preserves replay floors but does not approve their age or provenance.
func InspectStateDirectory(ctx context.Context, path string) (resultErr error) {
	if ctx == nil {
		return sourceValidation("state", nil, "inspection requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	directory, err := privatefiles.ExistingDirectory(path)
	if err != nil {
		return err
	}
	if err := directory.CheckNoPendingPublications(ctx); err != nil {
		return err
	}
	root, err := directory.Open()
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, root.Close()) }()
	listing, err := root.Open(".")
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, listing.Close()) }()
	count := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, readErr := listing.ReadDir(128)
		count += len(entries)
		if count > stateInspectionEntryLimit {
			return sourceValidation("state", nil, "inspection exceeds the entry limit")
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.Name() == privatefiles.PublicationDirectoryName && entry.IsDir() {
				continue
			}
			if !entry.Type().IsRegular() {
				return sourceValidation("state", nil, "inspection requires regular discovery records")
			}
			data, err := directory.ReadFile(entry.Name(), maxStateBytes)
			if err != nil {
				return err
			}
			state, err := inspectStateRecord(data)
			if err != nil {
				return err
			}
			digest := sha256.Sum256([]byte(state.Repository + "\x00" + state.Channel))
			if entry.Name() != hex.EncodeToString(digest[:])+stateFileSuffix {
				return sourceValidation("state", nil, "record name differs from its repository and channel")
			}
		}
		if stderrors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err := directory.CheckNoPendingPublications(ctx); err != nil {
		return err
	}
	verified, err := directory.Open()
	if err != nil {
		return err
	}
	return stderrors.Join(verified.Close(), ctx.Err())
}

func inspectStateRecord(data []byte) (State, error) {
	var state State
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return state, errors.WrapParse("discovery state", "", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return state, sourceValidation("state", nil, "requires one complete discovery record")
	}
	if state.SchemaVersion != StateSchemaVersion {
		return state, sourceValidation("state.schema_version", state.SchemaVersion, "is unsupported")
	}
	owner, repository, found := strings.Cut(state.Repository, "/")
	if !found || owner == "" || repository == "" || strings.Contains(repository, "/") || state.Channel == "" {
		return state, sourceValidation("state", nil, "requires a repository and channel")
	}
	for _, value := range []string{state.Repository, state.Channel, state.Verified.GenerationID} {
		if value == "" || strings.TrimSpace(value) != value || strings.ContainsFunc(value, unicode.IsControl) {
			return state, sourceValidation("state", nil, "contains an invalid identity")
		}
	}
	if state.Sequence == 0 || !artifact.IsReleaseTag(state.Verified.Tag) || !validStateDigest(trimChecksum(state.Verified.CatalogDigest)) {
		return state, sourceValidation("state", nil, "requires a replay sequence and verified release")
	}
	if state.ChannelChecksum != "" && !validStateDigest(state.ChannelChecksum) {
		return state, sourceValidation("state.channel_checksum", nil, "is invalid")
	}
	if state.Verified.VerifiedAt.IsZero() || state.UpdatedAt.IsZero() || state.UpdatedAt.Before(state.Verified.VerifiedAt) {
		return state, sourceValidation("state", nil, "requires consistent verification timestamps")
	}
	canonical, err := encodeState(state)
	if err != nil {
		return state, err
	}
	if !bytes.Equal(data, canonical) {
		return state, sourceValidation("state", nil, "requires canonical discovery encoding")
	}
	return state, nil
}

func validStateDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && hex.EncodeToString(decoded) == value
}
