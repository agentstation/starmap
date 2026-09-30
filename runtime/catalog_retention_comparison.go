package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	stderrors "errors"
	"fmt"
)

// CatalogRetentionComparison binds structurally checked original input to its canonical envelope.
// It retains private hashes and byte counts. It grants no ownership, selection, replay, or permission.
// Only InspectCatalogRetentionComparison can create a usable value.
type CatalogRetentionComparison struct {
	entry    CatalogRetentionEntry
	checksum string
	bytes    int64
	usage    CatalogRetentionUsage
}

// Format excludes private entry identities and envelope hashes from diagnostics.
func (p CatalogRetentionComparison) Format(state fmt.State, _ rune) {
	_, _ = fmt.Fprint(state, "CatalogRetentionComparison{private}")
}

// Usage returns the immutable byte counts from the original complete structural check.
// A nil or empty comparison returns zero counts.
func (p *CatalogRetentionComparison) Usage() CatalogRetentionUsage {
	if p == nil {
		return CatalogRetentionUsage{}
	}
	return p.usage
}

// InspectCatalogRetentionComparison checks original input and binds its canonical compressed envelope.
// The result contains no original data and has no serialized constructor.
// This inspection writes no files and contacts no sources.
func InspectCatalogRetentionComparison(ctx context.Context, entry CatalogRetentionEntry, input CatalogRetentionInput) (*CatalogRetentionComparison, error) {
	usage, err := InspectCatalogRetentionUsage(ctx, entry, input)
	if err != nil {
		return nil, err
	}
	encoded, err := encodeRetainedCatalogInput(entry, input)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &CatalogRetentionComparison{entry: entry, checksum: fleetRecoveryChecksum(encoded), bytes: int64(len(encoded)), usage: usage}, nil
}

// CheckRetainedCatalogRecovery compares current retained bytes with checked original input.
// It checks the native owner, completed batch, manifest, exact entry, and bounded envelope hash.
// It does not decode unchanged input, repair files, select a catalog, or establish current permission.
func CheckRetainedCatalogRecovery(ctx context.Context, request CatalogRetainedReadRequest, comparison *CatalogRetentionComparison) (resultErr error) {
	if comparison == nil || comparison.usage.Inputs() != 1 || !validFleetChecksum(comparison.checksum) || comparison.bytes <= 0 || comparison.bytes > MaxCatalogRetentionRecordBytes {
		return invalidInputPublication("retained comparison requires checked original input")
	}
	directory, lock, reference, err := openRetainedCatalogRecord(ctx, request)
	if err != nil {
		return err
	}
	defer func() { resultErr = stderrors.Join(resultErr, lock.Close()) }()
	if reference.Entry != comparison.entry || reference.RecordSHA256 != comparison.checksum {
		return invalidInputPublication("retained comparison differs from its original entry or envelope")
	}
	hash := sha256.New()
	size, err := directory.CopyFile(ctx, reference.RecordSHA256+".json.gz", hash, comparison.bytes)
	if err != nil {
		return err
	}
	if size != comparison.bytes || hex.EncodeToString(hash.Sum(nil)) != comparison.checksum {
		return invalidInputPublication("retained comparison envelope changed")
	}
	return ctx.Err()
}
