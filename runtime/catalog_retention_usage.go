package runtime

import (
	"context"

	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

// These bounds apply independently to raw capsule bytes, decoded recovery bytes, and input count.
const (
	MaxCatalogRetentionBatchBytes  = storage.DefaultRetentionInputMaxBytes
	MaxCatalogRetentionBatchInputs = storage.DefaultRetentionScanEntries
)

// CatalogRetentionUsage contains immutable byte counts without catalog data or private identities.
// It grants no retention, selection, current permission, or target ownership.
type CatalogRetentionUsage struct {
	rawBytes, decodedBytes int64
}

// RawBytes counts the serialized manifest, payload, compressed recovery, and original descriptor bytes.
func (u CatalogRetentionUsage) RawBytes() int64 { return u.rawBytes }

// DecodedBytes counts the decompressed original recovery record before JSON parsing.
func (u CatalogRetentionUsage) DecodedBytes() int64 { return u.decodedBytes }

// Inputs reports the one structurally checked input. An empty result reports zero.
func (u CatalogRetentionUsage) Inputs() int {
	if u.rawBytes == 0 || u.decodedBytes == 0 {
		return 0
	}
	return 1
}

// InspectCatalogRetentionUsage checks one capsule through the producer's original codecs.
// Hosts can pack against both byte dimensions and the count bound without another decoder.
// RetainCatalogRecovery independently checks each final batch. This inspection writes no files and contacts no sources.
func InspectCatalogRetentionUsage(ctx context.Context, entry CatalogRetentionEntry, input CatalogRetentionInput) (CatalogRetentionUsage, error) {
	return inspectCatalogRetentionUsage(ctx, entry, input, MaxCatalogRetentionBatchBytes, MaxCatalogRetentionBatchBytes)
}

func inspectCatalogRetentionUsage(ctx context.Context, entry CatalogRetentionEntry, input CatalogRetentionInput, rawLimit, decodedLimit int64) (CatalogRetentionUsage, error) {
	var usage CatalogRetentionUsage
	if ctx == nil || rawLimit <= 0 || rawLimit > MaxCatalogRetentionBatchBytes || decodedLimit <= 0 || decodedLimit > MaxCatalogRetentionBatchBytes {
		return usage, invalidInputPublication("retention usage requires bounded raw and decoded limits")
	}
	if err := ctx.Err(); err != nil {
		return usage, err
	}
	if err := validateCatalogRetentionInputBinding(entry, input); err != nil {
		return usage, err
	}
	manifest, err := marshalCatalogRetention(input.Generation.Manifest, storage.MaxFilesystemManifestBytes)
	if err != nil {
		return usage, err
	}
	for _, size := range []int{len(manifest), len(input.Generation.Payload), len(input.Recovery.Inputs.Data), len(input.SourceDescriptor)} {
		if int64(size) > rawLimit-usage.rawBytes {
			return CatalogRetentionUsage{}, invalidInputPublication("retention capsule exceeds its raw byte limit")
		}
		usage.rawBytes += int64(size)
	}
	decoded, err := decompressFleetRecovery(ctx, input.Recovery.Inputs.Data, decodedLimit)
	if err != nil {
		return CatalogRetentionUsage{}, err
	}
	record, err := decodeFleetRecoveryData(ctx, decoded)
	if err != nil {
		return CatalogRetentionUsage{}, err
	}
	if err := validateCatalogRetentionInputRecord(ctx, entry, input, record); err != nil {
		return CatalogRetentionUsage{}, err
	}
	if err := ctx.Err(); err != nil {
		return CatalogRetentionUsage{}, err
	}
	usage.decodedBytes = int64(len(decoded))
	return usage, nil
}
