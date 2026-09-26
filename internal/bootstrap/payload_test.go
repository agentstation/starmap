package bootstrap

import (
	"bytes"
	"encoding/json/v2"
	"testing"
	"testing/fstest"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestCompiledBootstrapRejectsInvalidInput(t *testing.T) {
	catalog, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatal(err)
	}
	semantic, err := catalogs.CatalogSemanticChecksum(catalog)
	if err != nil {
		t.Fatal(err)
	}
	baseline := catalogs.BootstrapManifest{
		ManifestVersion: catalogs.CurrentBootstrapManifestVersion, SchemaVersion: catalogs.CurrentCatalogSchemaVersion,
		GenerationID: "compiled-fixture", GeneratedAt: time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC),
		Payload: catalogs.DescribeCatalogPayload(payload), SemanticChecksum: semantic,
	}
	for _, kind := range []string{"valid", "missing_payload", "missing_manifest", "invalid_gzip", "truncated", "bad_crc", "trailing", "second_member", "checksum", "short_size", "long_size", "oversize", "invalid_json", "noncanonical", "semantic", "schema"} {
		t.Run(kind, func(t *testing.T) {
			manifest := baseline
			encoded := payload
			switch kind {
			case "invalid_json":
				encoded = []byte("invalid")
				manifest.Payload = catalogs.DescribeCatalogPayload(encoded)
			case "noncanonical":
				encoded = append(bytes.Clone(payload), '\n')
				manifest.Payload = catalogs.DescribeCatalogPayload(encoded)
			case "checksum":
				manifest.Payload.Checksum = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			case "short_size":
				manifest.Payload.SizeBytes--
			case "long_size":
				manifest.Payload.SizeBytes++
			case "oversize":
				manifest.Payload.SizeBytes = catalogs.MaxCatalogPayloadBytes + 1
			case "semantic":
				manifest.SemanticChecksum = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			case "schema":
				manifest.SchemaVersion++
			}
			compressed, err := EncodePayload(encoded)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "invalid_gzip":
				compressed = []byte("invalid")
			case "truncated":
				compressed = compressed[:len(compressed)-1]
			case "bad_crc":
				compressed[len(compressed)-8] ^= 1
			case "trailing":
				compressed = append(compressed, 0)
			case "second_member":
				compressed = append(compressed, compressed...)
			}
			metadata, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			files := fstest.MapFS{
				"generation.json": {Data: metadata}, PayloadFilename: {Data: compressed},
				"providers.yaml": {Data: []byte("[]\n")},
			}
			if kind == "missing_payload" {
				delete(files, PayloadFilename)
			}
			if kind == "missing_manifest" {
				delete(files, "generation.json")
			}
			got, actual, err := loadCompiled(files)
			if kind == "valid" {
				if err != nil || got == nil || actual != baseline {
					t.Fatalf("valid compiled catalog: %v, %#v", err, actual)
				}
			} else if err == nil || got != nil {
				t.Fatalf("invalid compiled catalog became usable: %v", err)
			}
		})
	}
}

func TestCompiledPayloadEncodingIsDeterministicAndBounded(t *testing.T) {
	payload := []byte("canonical fixture")
	first, err := EncodePayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EncodePayload(payload)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("compression changed bytes: %v", err)
	}
	for _, size := range []int{0, catalogs.MaxCatalogPayloadBytes + 1} {
		if _, err := EncodePayload(make([]byte, size)); err == nil {
			t.Fatalf("accepted invalid payload size %d", size)
		}
	}
	for _, size := range []int64{-1, 0, catalogs.MaxCatalogPayloadBytes + 1, 1<<63 - 1} {
		descriptor := catalogs.DescribeCatalogPayload(payload)
		descriptor.SizeBytes = size
		if _, err := DecodePayload(first, descriptor); err == nil {
			t.Fatalf("accepted invalid descriptor size %d", size)
		}
	}
}
