package bootstrap

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

// PayloadFilename names the generated compressed canonical catalog.
const PayloadFilename = "generation-payload.json.gz"

// EncodePayload compresses canonical bytes with deterministic gzip metadata.
func EncodePayload(payload []byte) ([]byte, error) {
	if len(payload) == 0 || len(payload) > catalogs.MaxCatalogPayloadBytes {
		return nil, &errors.ValidationError{Field: "bootstrap_payload.size", Message: "exceeds the catalog payload bounds"}
	}
	var output bytes.Buffer
	writer := gzip.NewWriter(&output)
	if _, err := writer.Write(payload); err != nil {
		return nil, errors.WrapIO("compress", PayloadFilename, err)
	}
	if err := writer.Close(); err != nil {
		return nil, errors.WrapIO("compress", PayloadFilename, err)
	}
	return output.Bytes(), nil
}

// DecodePayload bounds decompression and verifies the exact canonical byte descriptor.
func DecodePayload(compressed []byte, descriptor catalogs.PayloadDescriptor) ([]byte, error) {
	if descriptor.SizeBytes <= 0 || descriptor.SizeBytes > catalogs.MaxCatalogPayloadBytes {
		return nil, &errors.ValidationError{Field: "bootstrap_payload.size", Message: "exceeds the catalog payload bounds"}
	}
	input := bytes.NewReader(compressed)
	reader, err := gzip.NewReader(input)
	if err != nil {
		return nil, errors.WrapIO("decompress", PayloadFilename, err)
	}
	defer func() { _ = reader.Close() }()
	reader.Multistream(false)
	payload, err := io.ReadAll(io.LimitReader(reader, descriptor.SizeBytes+1))
	if err != nil {
		return nil, errors.WrapIO("decompress", PayloadFilename, err)
	}
	if input.Len() != 0 {
		return nil, &errors.ValidationError{Field: "bootstrap_payload.gzip", Message: "contains trailing data"}
	}
	if err := descriptor.Verify(payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func loadCompiled(catalogFS fs.FS) (*catalogs.Catalog, catalogs.BootstrapManifest, error) {
	data, err := fs.ReadFile(catalogFS, "generation.json")
	if err != nil {
		return nil, catalogs.BootstrapManifest{}, errors.WrapIO("read", manifestPath, err)
	}
	manifest, err := catalogs.ParseBootstrapManifestJSON(data)
	if err != nil {
		return nil, catalogs.BootstrapManifest{}, err
	}
	compressed, err := fs.ReadFile(catalogFS, PayloadFilename)
	if err != nil {
		return nil, catalogs.BootstrapManifest{}, errors.WrapIO("read", PayloadFilename, err)
	}
	payload, err := DecodePayload(compressed, manifest.Payload)
	if err != nil {
		return nil, catalogs.BootstrapManifest{}, err
	}
	catalog, err := catalogs.DecodeCatalogPayload(payload)
	if err != nil {
		return nil, catalogs.BootstrapManifest{}, err
	}
	if err := verifyCatalog(catalog, manifest); err != nil {
		return nil, catalogs.BootstrapManifest{}, err
	}
	return catalog, manifest, nil
}
