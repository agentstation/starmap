package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"
	"github.com/agentstation/starmap/internal/privatefiles"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

// RetainedRecordReader reads a record from a verified backup with the supplied byte limit.
// An absent record must return an error that matches os.ErrNotExist.
type RetainedRecordReader = privatefiles.RetainedRecordReader

// InspectRetainedMigration identifies completed migration records to keep inactive during restore.
// The caller supplies the captured directory name, configured owner and explicit scheduler identity.
// Paths in historical records are opaque identities. This check never opens those paths.
// The caller must verify the complete backup and the remaining runtime tree separately.
// This result does not approve current permission, external fencing, or replica reuse.
func InspectRetainedMigration(ctx context.Context, originalDirectory string, owner DirectoryOwner, identity string, read RetainedRecordReader) ([]string, error) {
	if ctx == nil || read == nil {
		return nil, invalidMigrationIntent("restore_reader")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := owner.Validate(); err != nil {
		return nil, err
	}
	receipt, receiptErr := read(ctx, migrationReceiptName, migrationManifestMaxBytes)
	completion, completionErr := read(ctx, migrationCompletionName, migrationManifestMaxBytes)
	if stderrors.Is(receiptErr, os.ErrNotExist) && stderrors.Is(completionErr, os.ErrNotExist) {
		return nil, ctx.Err()
	}
	if receiptErr != nil {
		return nil, receiptErr
	}
	if completionErr != nil {
		return nil, completionErr
	}
	if len(receipt) > migrationManifestMaxBytes || !bytes.Equal(receipt, completion) {
		return nil, invalidMigrationIntent("restore_completion")
	}
	manifest, err := decodeRetainedMigration(receipt)
	if err != nil {
		return nil, err
	}
	if manifest.Owner != owner || identity == "" || manifest.SourceIdentity != identity || manifest.TargetDirectory != originalDirectory {
		return nil, invalidMigrationIntent("restore_identity")
	}
	seed, err := read(ctx, instanceSeedFileName, int64(hex.EncodedLen(instanceSeedBytes)))
	if err != nil {
		return nil, err
	}
	if len(seed) != hex.EncodedLen(instanceSeedBytes) {
		return nil, invalidMigrationIntent("restore_seed")
	}
	decoded, err := hex.DecodeString(string(seed))
	if err != nil || len(decoded) != instanceSeedBytes || hex.EncodeToString(decoded) != string(seed) {
		return nil, invalidMigrationIntent("restore_seed")
	}
	digest := sha256.Sum256(seed)
	for _, file := range manifest.Files {
		if file.Target == instanceSeedFileName && (file.Size != int64(len(seed)) || file.SHA256 != hex.EncodeToString(digest[:])) {
			return nil, invalidMigrationIntent("restore_seed")
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []string{migrationReceiptName, migrationCompletionName}, nil
}

func decodeRetainedMigration(receipt []byte) (directoryMigrationManifest, error) {
	var manifest directoryMigrationManifest
	if err := decodeInputRecord(receipt, &manifest); err != nil {
		return manifest, err
	}
	if err := manifest.validateIdentity(); err != nil {
		return manifest, err
	}
	// Do not interpret a former host's paths using this host's filesystem rules.
	for _, value := range []string{manifest.SourceDirectory, manifest.TargetDirectory} {
		if value == "" || len(value) > 4096 || !utf8.ValidString(value) || strings.TrimSpace(value) != value || strings.ContainsFunc(value, unicode.IsControl) {
			return manifest, invalidMigrationIntent("restore_directory")
		}
	}
	if manifest.SourceDirectory == manifest.TargetDirectory {
		return manifest, invalidMigrationIntent("restore_directory")
	}
	canonical, err := manifest.encodeFiles()
	if err != nil {
		return manifest, err
	}
	if !bytes.Equal(receipt, canonical) {
		return manifest, invalidMigrationIntent("restore_encoding")
	}
	return manifest, nil
}
