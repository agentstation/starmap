package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestCatalogRetentionUsageMatchesActualCodecAndRetainsNothing(t *testing.T) {
	request, _, _ := materializationFixture(t)
	input := CatalogRetentionInput{CatalogMaterializationInput: request.Inputs[0]}
	entry := CatalogRetentionEntry{ManifestSHA256: input.Recovery.ManifestChecksum, InputsSHA256: input.Recovery.Inputs.Checksum, SourceOrigin: CatalogRetentionNoDescriptor}
	store, err := existingLayerStore(request.Directory)
	if err != nil {
		t.Fatal(err)
	}
	before, err := captureMaterializationFiles(t.Context(), store.directory)
	if err != nil {
		t.Fatal(err)
	}
	usage, err := InspectCatalogRetentionUsage(t.Context(), entry, input)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := marshalCatalogRetention(input.Generation.Manifest, MaxCatalogRetentionBatchBytes)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decompressFleetRecovery(t.Context(), input.Recovery.Inputs.Data, MaxCatalogRetentionBatchBytes)
	if err != nil {
		t.Fatal(err)
	}
	want := int64(len(manifest) + len(input.Generation.Payload) + len(input.Recovery.Inputs.Data))
	if usage.RawBytes() != want || usage.DecodedBytes() != int64(len(decoded)) || usage.Inputs() != 1 {
		t.Fatalf("usage=%v want raw=%d decoded=%d count=1", usage, want, len(decoded))
	}
	after, err := captureMaterializationFiles(t.Context(), store.directory)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("passive usage changed selected files", err)
	}
	for _, verb := range []string{"%v", "%+v", "%#v", "%p"} {
		if strings.Contains(fmt.Sprintf(verb, usage), request.Directory) {
			t.Fatal("usage diagnostics exposed private native scope")
		}
	}
	for _, limits := range [][2]int64{{usage.RawBytes(), usage.DecodedBytes()}, {MaxCatalogRetentionBatchBytes, MaxCatalogRetentionBatchBytes}} {
		if got, err := inspectCatalogRetentionUsage(t.Context(), entry, input, limits[0], limits[1]); err != nil || got != usage {
			t.Fatal("exact independent usage limits refused", got, err)
		}
	}
	for _, limits := range [][2]int64{{usage.RawBytes() - 1, MaxCatalogRetentionBatchBytes}, {MaxCatalogRetentionBatchBytes, usage.DecodedBytes() - 1}, {0, 1}, {-1, 1}, {1, 0}, {1, -1}, {MaxCatalogRetentionBatchBytes + 1, 1}, {1, MaxCatalogRetentionBatchBytes + 1}} {
		if got, err := inspectCatalogRetentionUsage(t.Context(), entry, input, limits[0], limits[1]); err == nil || got != (CatalogRetentionUsage{}) {
			t.Fatal("invalid limits produced a usable report", limits, got, err)
		}
	}
}

func TestCatalogRetentionUsageRefusesUncertainOriginalBytes(t *testing.T) {
	request, _, _ := materializationFixture(t)
	original := CatalogRetentionInput{CatalogMaterializationInput: request.Inputs[0]}
	originalEntry := CatalogRetentionEntry{ManifestSHA256: original.Recovery.ManifestChecksum, InputsSHA256: original.Recovery.Inputs.Checksum, SourceOrigin: CatalogRetentionNoDescriptor}
	for _, field := range []string{"empty", "manifest", "recovery", "source-origin", "descriptor", "compressed", "schema", "trailing"} {
		t.Run(field, func(t *testing.T) {
			input, entry := original, originalEntry
			switch field {
			case "empty":
				input = CatalogRetentionInput{}
			case "manifest":
				entry.ManifestSHA256 = strings.Repeat("a", 64)
			case "recovery":
				entry.InputsSHA256 = strings.Repeat("a", 64)
			case "source-origin":
				entry.SourceOrigin = "unknown"
			case "descriptor":
				input.SourceDescriptor = []byte("unexpected")
			case "compressed":
				input.Recovery.Inputs.Data = []byte("invalid compression")
			case "schema":
				var err error
				input.Recovery.Inputs.Data, err = compressFleetRecovery([]byte(`{"version":999}`))
				if err != nil {
					t.Fatal(err)
				}
			case "trailing":
				input.Recovery.Inputs.Data = append(bytes.Clone(input.Recovery.Inputs.Data), 0)
			}
			if field == "compressed" || field == "schema" || field == "trailing" {
				input.Recovery.Inputs.Checksum = fleetRecoveryChecksum(input.Recovery.Inputs.Data)
				entry.InputsSHA256 = input.Recovery.Inputs.Checksum
			}
			if got, err := InspectCatalogRetentionUsage(t.Context(), entry, input); err == nil || got != (CatalogRetentionUsage{}) {
				t.Fatal("invalid original bytes produced a usable report", got, err)
			}
		})
	}
	if got, err := InspectCatalogRetentionUsage(nil, originalEntry, original); err == nil || got.Inputs() != 0 {
		t.Fatal("nil context produced a usable report", got, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if got, err := InspectCatalogRetentionUsage(ctx, originalEntry, original); !errors.Is(err, context.Canceled) || got.Inputs() != 0 {
		t.Fatal("cancelled inspection produced a usable report", got, err)
	}
}

func TestCatalogRetentionUsageHighlyCompressedBatchUsesBothDimensions(t *testing.T) {
	request, _ := retentionFixture(t, 2)
	var raw, decoded int64
	for i := range request.Inputs {
		input := &request.Inputs[i]
		record, err := readFleetRecovery(t.Context(), input.Recovery.Inputs.Data)
		if err != nil {
			t.Fatal(err)
		}
		record.PublisherID = strings.Repeat("historic-fact-lineage", 4096)
		input.Recovery.Inputs.Data, err = encodeFleetRecoveryRecord(record)
		if err != nil {
			t.Fatal(err)
		}
		input.Recovery.Inputs.Checksum = fleetRecoveryChecksum(input.Recovery.Inputs.Data)
		request.Manifest[i].InputsSHA256 = input.Recovery.Inputs.Checksum
		usage, err := InspectCatalogRetentionUsage(t.Context(), request.Manifest[i], *input)
		if err != nil {
			t.Fatal(err)
		}
		raw += usage.RawBytes()
		decoded += usage.DecodedBytes()
	}
	if decoded <= raw*10 || raw > MaxCatalogRetentionBatchBytes || decoded > MaxCatalogRetentionBatchBytes {
		t.Fatal("fixture did not exercise independent compressed and decoded dimensions", raw, decoded)
	}
	if receipt, err := RetainCatalogRecovery(t.Context(), request); err != nil || len(receipt.Records) != 2 {
		t.Fatal("legal measured batch did not retain exact capsules", receipt, err)
	}
	if MaxCatalogRetentionBatchInputs != 4096 || (CatalogRetentionUsage{}).Inputs() != 0 {
		t.Fatal("count contract differs from the native retention manifest")
	}
}
