package artifact

import (
	"bytes"
	"encoding/json"
	"io"
)

type publicationReceiptHeader PublicationReceipt

func decodePublicationReceipt(data []byte) (PublicationReceipt, error) {
	var header publicationReceiptHeader
	wire := struct {
		*publicationReceiptHeader
		Sources json.RawMessage `json:"sources"`
	}{publicationReceiptHeader: &header}
	if err := decodeStrictJSON(data, &wire); err != nil {
		return PublicationReceipt{}, publicationReceiptError("document", "requires a receipt with known JSON fields")
	}
	sources, err := decodePublicationSources(wire.Sources)
	if err != nil {
		return PublicationReceipt{}, err
	}
	header.Sources = sources
	return PublicationReceipt(header), nil
}

func decodePublicationSources(data []byte) ([]PublicationSourceReceipt, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if token, err := decoder.Token(); err != nil || token != json.Delim('[') {
		return nil, publicationReceiptError("sources", "requires a source array")
	}
	var sources []PublicationSourceReceipt
	for decoder.More() {
		if len(sources) >= maxPublicationScopes {
			return nil, publicationReceiptError("sources", "exceeds the source count limit")
		}
		var source PublicationSourceReceipt
		if err := decoder.Decode(&source); err != nil {
			return nil, publicationReceiptError("sources", "requires source receipts with known JSON fields")
		}
		sources = append(sources, source)
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim(']') {
		return nil, publicationReceiptError("sources", "requires a complete source array")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, publicationReceiptError("sources", "requires one source array")
	}
	return sources, nil
}
