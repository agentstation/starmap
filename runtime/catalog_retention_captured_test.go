package runtime

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCapturedCatalogRetentionReadsOriginalImmutableEnvelope(t *testing.T) {
	request, _ := retentionFixture(t, 1)
	receipt, err := RetainCatalogRecovery(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	name := receipt.Records[0].RecordSHA256 + ".json.gz"
	file := filepath.Join(request.Directory, layerDirectoryName, retainedCatalogDirectory, fleetRecoveryChecksum([]byte(request.TransferID)), name)
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	entry, input, err := InspectCapturedCatalogRetention(t.Context(), name, raw)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := encodeRetainedCatalogInput(entry, input)
	if err != nil {
		t.Fatal(err)
	}
	if entry != receipt.Records[0].Entry || !bytes.Equal(encoded, raw) {
		t.Fatal("original capsule bytes changed")
	}
	originalBytes := bytes.Clone(raw)
	clear(raw)
	encoded, err = encodeRetainedCatalogInput(entry, input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, originalBytes) {
		t.Fatal("caller changed decoded original")
	}
	for _, invalid := range []string{"../" + name, name + ".extra", strings.Repeat("a", 64) + ".json.gz"} {
		if _, _, err := InspectCapturedCatalogRetention(t.Context(), invalid, []byte("invalid")); err == nil {
			t.Fatal("invalid immutable identity accepted")
		}
	}
	for _, body := range [][]byte{nil, []byte("not gzip")} {
		name := fleetRecoveryChecksum(body) + ".json.gz"
		if _, _, err := InspectCapturedCatalogRetention(t.Context(), name, body); err == nil {
			t.Fatal("invalid envelope accepted")
		}
	}
	original, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := append(bytes.Clone(original), 1)
	if _, _, err := InspectCapturedCatalogRetention(t.Context(), fleetRecoveryChecksum(corrupt)+".json.gz", corrupt); err == nil {
		t.Fatal("trailing data accepted")
	}
	for _, change := range []func(*retainedCatalogEnvelope){
		func(e *retainedCatalogEnvelope) { e.Version++ },
		func(e *retainedCatalogEnvelope) { e.Entry.InputsSHA256 = strings.Repeat("a", 64) },
		func(e *retainedCatalogEnvelope) { e.Input.SourceDescriptor = []byte("unexpected") },
		func(e *retainedCatalogEnvelope) { e.Input.Generation.Manifest.GenerationID += "-changed" },
	} {
		envelope := retainedCatalogEnvelope{Version: materializationVersion, Entry: entry, Input: input}
		change(&envelope)
		data, err := marshalCatalogRetention(envelope, maxRetainedCatalogRecordBytes)
		if err != nil {
			t.Fatal(err)
		}
		compressed, err := compressRecoveryRecord(data, maxRetainedCatalogRecordBytes)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := InspectCapturedCatalogRetention(t.Context(), fleetRecoveryChecksum(compressed)+".json.gz", compressed); err == nil {
			t.Fatal("changed envelope accepted")
		}
	}
	data, err := json.Marshal(map[string]any{"version": materializationVersion, "entry": entry, "input": input, "unknown": true})
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := compressRecoveryRecord(data, maxRetainedCatalogRecordBytes)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := InspectCapturedCatalogRetention(t.Context(), fleetRecoveryChecksum(compressed)+".json.gz", compressed); err == nil {
		t.Fatal("unknown envelope member accepted")
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := InspectCapturedCatalogRetention(canceled, name, original); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, _, err := InspectCapturedCatalogRetention(nil, name, original); err == nil {
		t.Fatal("nil context accepted")
	}
}
