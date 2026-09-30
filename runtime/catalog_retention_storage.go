package runtime

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"maps"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

const retainedCatalogDirectory = "retained-catalog"

// JSON base64 expansion fits within twice the bounded raw input inventory.
const maxRetainedCatalogRecordBytes = 2 * storage.DefaultRetentionInputMaxBytes

// MaxCatalogRetentionRecordBytes bounds encoded and decoded original retention envelopes.
// JSON encoding can expand raw capsule bytes to twice the raw input bound.
const MaxCatalogRetentionRecordBytes = maxRetainedCatalogRecordBytes

type retainedCatalogManifest struct {
	Version    int                     `json:"version"`
	TransferID string                  `json:"transfer_id"`
	Entries    []CatalogRetentionEntry `json:"entries"`
}
type retainedCatalogEnvelope struct {
	Version int                   `json:"version"`
	Entry   CatalogRetentionEntry `json:"entry"`
	Input   CatalogRetentionInput `json:"input"`
}

func validRecoveryOperationID(value string) bool {
	return value != "" && len(value) <= deploymentIDMaxBytes && strings.TrimSpace(value) == value && utf8.ValidString(value) && !strings.ContainsFunc(value, unicode.IsControl)
}
func sortedMaterializationNames(files map[string][]byte) []string {
	return slices.Sorted(maps.Keys(files))
}
func validateCatalogRetentionEntry(entry CatalogRetentionEntry) error {
	if !validFleetChecksum(entry.ManifestSHA256) || !validFleetChecksum(entry.InputsSHA256) {
		return invalidInputPublication("retention requires exact generation and exported input digests")
	}
	switch entry.SourceOrigin {
	case CatalogRetentionLocalDescriptor, CatalogRetentionFleetDescriptor:
		if !validFleetChecksum(entry.SourceDescriptorSHA256) {
			return invalidInputPublication("retention origin requires its exact source descriptor")
		}
	case CatalogRetentionNoDescriptor:
		if entry.SourceDescriptorSHA256 != "" {
			return invalidInputPublication("absent descriptor origin contains a descriptor identity")
		}
	default:
		return invalidInputPublication("retention has unknown source descriptor origin")
	}
	return nil
}
func catalogRetentionManifestBytes(transfer string, entries []CatalogRetentionEntry) ([]byte, error) {
	if !validRecoveryOperationID(transfer) || len(entries) == 0 || len(entries) > storage.DefaultRetentionScanEntries {
		return nil, invalidInputPublication("retention requires a bounded complete transfer manifest")
	}
	seen := map[CatalogRetentionEntry]bool{}
	for _, entry := range entries {
		if err := validateCatalogRetentionEntry(entry); err != nil {
			return nil, err
		}
		if seen[entry] {
			return nil, invalidInputPublication("retention manifest repeats an input")
		}
		seen[entry] = true
	}
	return json.Marshal(retainedCatalogManifest{Version: materializationVersion, TransferID: transfer, Entries: entries}, json.Deterministic(true))
}
func validateCatalogRetentionInput(ctx context.Context, entry CatalogRetentionEntry, input CatalogRetentionInput) error {
	if err := validateCatalogRetentionInputBinding(entry, input); err != nil {
		return err
	}
	record, err := readFleetRecovery(ctx, input.Recovery.Inputs.Data)
	if err != nil {
		return err
	}
	return validateCatalogRetentionInputRecord(ctx, entry, input, record)
}

func validateCatalogRetentionInputBinding(entry CatalogRetentionEntry, input CatalogRetentionInput) error {
	if err := validateCatalogRetentionEntry(entry); err != nil {
		return err
	}
	if input.Recovery.ManifestChecksum != entry.ManifestSHA256 || input.Recovery.Inputs.Checksum != entry.InputsSHA256 {
		return invalidInputPublication("retention input differs from its manifest")
	}
	if err := input.Recovery.Validate(input.Generation); err != nil {
		return err
	}
	return nil
}

func validateCatalogRetentionInputRecord(ctx context.Context, entry CatalogRetentionEntry, input CatalogRetentionInput, record fleetRecoveryRecord) error {
	if record.Pin != nil && !pinRecordMatches(*record.Pin, input.Generation) {
		return pinRecordConflict("retained capsule selects another pinned generation")
	}
	if _, err := decodeFleetRecoveryRecord(ctx, record, layerSet{}); err != nil {
		return err
	}
	if entry.SourceOrigin == CatalogRetentionNoDescriptor {
		if len(input.SourceDescriptor) != 0 {
			return invalidInputPublication("absent descriptor origin contains source bytes")
		}
		return nil
	}
	if len(input.SourceDescriptor) == 0 || len(input.SourceDescriptor) > MaxFleetRecoveryBytes || fleetRecoveryChecksum(input.SourceDescriptor) != entry.SourceDescriptorSHA256 {
		return invalidInputPublication("retained source descriptor differs from its digest")
	}
	if entry.SourceOrigin == CatalogRetentionLocalDescriptor {
		decoded, err := decompressFleetRecovery(ctx, input.SourceDescriptor, MaxFleetRecoveryBytes)
		if err != nil {
			return err
		}
		var source localCatalogRecovery
		if err := json.Unmarshal(decoded, &source, json.RejectUnknownMembers(true), jsonv1.FormatDurationAsNano(true)); err != nil {
			return err
		}
		if err := source.validate(input.Generation); err != nil {
			return err
		}
		baseline, err := catalogManifestChecksum(record.Baseline)
		if err != nil {
			return err
		}
		record.Baseline = source.Inputs.Baseline
		if source.BaselineChecksum != baseline || !reflect.DeepEqual(source.Inputs, record) {
			return invalidInputPublication("local source descriptor differs from exported historical inputs")
		}
	}
	return ctx.Err()
}
func validateCatalogRetentionRequest(ctx context.Context, request CatalogRetentionRequest) ([]byte, []CatalogRetainedRecord, error) {
	if !filepath.IsAbs(request.Directory) || filepath.Clean(request.Directory) != request.Directory || !validRecoveryOperationID(request.OperationID) {
		return nil, nil, invalidInputPublication("retention requires an absolute directory and bounded operation identity")
	}
	if err := request.Owner.Validate(); err != nil {
		return nil, nil, err
	}
	if _, err := catalogRetentionManifestBytes(request.TransferID, request.Manifest); err != nil {
		return nil, nil, err
	}
	if len(request.Inputs) == 0 || request.BatchStart < 0 || len(request.Inputs) > len(request.Manifest) || request.BatchStart > len(request.Manifest)-len(request.Inputs) {
		return nil, nil, invalidInputPublication("retention batch exceeds its exact manifest range")
	}
	var total, decodedTotal int64
	records := make([]CatalogRetainedRecord, 0, len(request.Inputs))
	for i, input := range request.Inputs {
		entry := request.Manifest[request.BatchStart+i]
		usage, err := inspectCatalogRetentionUsage(ctx, entry, input, MaxCatalogRetentionBatchBytes-total, MaxCatalogRetentionBatchBytes-decodedTotal)
		if err != nil {
			return nil, nil, err
		}
		total += usage.rawBytes
		decodedTotal += usage.decodedBytes
		encoded, err := encodeRetainedCatalogInput(entry, input)
		if err != nil {
			return nil, nil, err
		}
		records = append(records, CatalogRetainedRecord{Entry: entry, RecordSHA256: fleetRecoveryChecksum(encoded)})
	}
	data, err := marshalCatalogRetention(request, maxRetainedCatalogRecordBytes)
	return data, records, err
}
func encodeRetainedCatalogInput(entry CatalogRetentionEntry, input CatalogRetentionInput) ([]byte, error) {
	raw, err := marshalCatalogRetention(retainedCatalogEnvelope{Version: materializationVersion, Entry: entry, Input: input}, maxRetainedCatalogRecordBytes)
	if err != nil {
		return nil, err
	}
	return compressRecoveryRecord(raw, maxRetainedCatalogRecordBytes)
}
func catalogTransferDirectory(store *layerStore, transfer string, create bool) (*privatefiles.Directory, error) {
	name := retainedCatalogDirectory + "/" + fleetRecoveryChecksum([]byte(transfer))
	directory, _, err := materializationFileDirectory(store.directory, name+"/manifest.json", create)
	return directory, err
}
func retainCatalogTransferManifest(ctx context.Context, store *layerStore, transfer string, entries []CatalogRetentionEntry) error {
	data, err := catalogRetentionManifestBytes(transfer, entries)
	if err != nil {
		return err
	}
	directory, err := catalogTransferDirectory(store, transfer, true)
	if err != nil {
		return err
	}
	if err := directory.RecoverPublications(ctx); err != nil {
		return err
	}
	prior, err := optionalMaterializationFile(directory, "manifest.json", maxLayerBytes)
	if err != nil {
		return err
	}
	if prior != nil && !bytes.Equal(prior, data) {
		return invalidInputPublication("catalog transfer manifest changed across batches")
	}
	return directory.CompareAndPublishFileContext(ctx, "manifest.json", prior, data, ".retained-manifest-")
}
func readRetainedCatalogInput(ctx context.Context, directory *privatefiles.Directory, reference CatalogRetainedRecord) (CatalogMaterializationInput, error) {
	input, err := readRetainedCatalogEnvelope(ctx, directory, reference)
	return input.CatalogMaterializationInput, err
}
func readRetainedCatalogEnvelope(ctx context.Context, directory *privatefiles.Directory, reference CatalogRetainedRecord) (CatalogRetentionInput, error) {
	raw, err := directory.ReadFile(reference.RecordSHA256+".json.gz", maxRetainedCatalogRecordBytes)
	if err != nil {
		return CatalogRetentionInput{}, err
	}
	entry, input, err := InspectCapturedCatalogRetention(ctx, reference.RecordSHA256+".json.gz", raw)
	if err != nil {
		return CatalogRetentionInput{}, err
	}
	if entry != reference.Entry {
		return CatalogRetentionInput{}, invalidInputPublication("retained envelope differs from its receipt")
	}
	return input, nil
}

// InspectCapturedCatalogRetention checks original immutable envelope bytes without opening a runtime.
// It checks structure and generation identity. It proves no completed batch, selection, ownership, or permission.
// The host must verify the complete original archive census and publication records separately.
func InspectCapturedCatalogRetention(ctx context.Context, name string, raw []byte) (CatalogRetentionEntry, CatalogRetentionInput, error) {
	if ctx == nil || len(raw) == 0 || len(raw) > MaxCatalogRetentionRecordBytes {
		return CatalogRetentionEntry{}, CatalogRetentionInput{}, invalidInputPublication("captured retention requires bounded original envelope bytes")
	}
	if err := ctx.Err(); err != nil {
		return CatalogRetentionEntry{}, CatalogRetentionInput{}, err
	}
	checksum, ok := strings.CutSuffix(name, ".json.gz")
	if !ok || !validFleetChecksum(checksum) || fleetRecoveryChecksum(raw) != checksum {
		return CatalogRetentionEntry{}, CatalogRetentionInput{}, invalidInputPublication("retained envelope differs from its immutable identity")
	}
	data, err := decompressRecoveryRecord(ctx, raw, maxRetainedCatalogRecordBytes, maxRetainedCatalogRecordBytes)
	if err != nil {
		return CatalogRetentionEntry{}, CatalogRetentionInput{}, err
	}
	var envelope retainedCatalogEnvelope
	if err := json.Unmarshal(data, &envelope, json.RejectUnknownMembers(true), jsonv1.FormatDurationAsNano(true)); err != nil {
		return CatalogRetentionEntry{}, CatalogRetentionInput{}, err
	}
	if envelope.Version != materializationVersion {
		return CatalogRetentionEntry{}, CatalogRetentionInput{}, invalidInputPublication("retained envelope has an unsupported version")
	}
	if err := validateCatalogRetentionInput(ctx, envelope.Entry, envelope.Input); err != nil {
		return CatalogRetentionEntry{}, CatalogRetentionInput{}, err
	}
	if err := ctx.Err(); err != nil {
		return CatalogRetentionEntry{}, CatalogRetentionInput{}, err
	}
	return envelope.Entry, envelope.Input, nil
}

func retainedCatalogFileLimit(name string) (int64, bool) {
	prefix := layerDirectoryName + "/" + retainedCatalogDirectory
	if path.Dir(path.Dir(name)) != prefix || !validFleetChecksum(path.Base(path.Dir(name))) {
		return 0, false
	}
	if path.Base(name) == "manifest.json" {
		return maxLayerBytes, true
	}
	base, ok := strings.CutSuffix(path.Base(name), ".json.gz")
	if ok && validFleetChecksum(base) {
		return maxRetainedCatalogRecordBytes, true
	}
	return 0, false
}
func retainedCatalogDirectoryName(name string) bool {
	prefix := layerDirectoryName + "/" + retainedCatalogDirectory
	return name == prefix || path.Dir(name) == prefix && validFleetChecksum(path.Base(name))
}

// retentionJSONBuffer stops encoded expansion at its fixed byte bound.
type retentionJSONBuffer struct {
	buffer  bytes.Buffer
	maximum int
}

func (b *retentionJSONBuffer) Write(data []byte) (int, error) {
	if len(data) > b.maximum-b.buffer.Len() {
		return 0, invalidInputPublication("retention JSON exceeds its encoded byte limit")
	}
	return b.buffer.Write(data)
}
func marshalCatalogRetention(value any, maximum int) ([]byte, error) {
	output := retentionJSONBuffer{maximum: maximum}
	if err := json.MarshalWrite(&output, value, json.Deterministic(true), jsonv1.FormatDurationAsNano(true)); err != nil {
		return nil, err
	}
	return output.buffer.Bytes(), nil
}

func checkCatalogTransferManifest(ctx context.Context, store *layerStore, transfer string, entries []CatalogRetentionEntry) error {
	expected, err := catalogRetentionManifestBytes(transfer, entries)
	if err != nil {
		return err
	}
	directory, err := catalogTransferDirectory(store, transfer, false)
	if err != nil {
		return err
	}
	actual, err := directory.ReadFile("manifest.json", maxLayerBytes)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return invalidInputPublication("catalog transfer manifest changed during retention")
	}
	return ctx.Err()
}
