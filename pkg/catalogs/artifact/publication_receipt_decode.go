package artifact

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
)

type publicationReceiptHeader PublicationReceipt

// UnmarshalJSON bounds quarantine records before decoding their fields.
func (s *PublicationSourceReceipt) UnmarshalJSON(data []byte) error {
	type sourceHeader PublicationSourceReceipt
	var header sourceHeader
	wire := struct {
		*sourceHeader
		Quarantine json.RawMessage `json:"quarantine"`
	}{sourceHeader: &header}
	if err := decodeStrictJSON(data, &wire); err != nil {
		return publicationReceiptError("source", "requires known source fields")
	}
	if len(wire.Quarantine) != 0 {
		var report struct {
			Records evidence.ObservationRecordCounts `json:"records"`
			Issues  json.RawMessage                  `json:"issues"`
		}
		if err := decodeStrictJSON(wire.Quarantine, &report); err != nil {
			return publicationReceiptError("source.quarantine", "requires classified record diagnostics")
		}
		issues, err := decodePublicationArray[evidence.QuarantinedRecord](report.Issues, "source.quarantine.issues")
		if err != nil {
			return err
		}
		header.Quarantine = &evidence.RecordQuarantine{Records: report.Records, Issues: issues}
	}
	*s = PublicationSourceReceipt(header)
	return nil
}

func decodePublicationReceipt(data []byte) (PublicationReceipt, error) {
	var header publicationReceiptHeader
	wire := struct {
		*publicationReceiptHeader
		Sources            json.RawMessage `json:"sources"`
		Reviews            json.RawMessage `json:"reviews"`
		ReviewObservations json.RawMessage `json:"review_observations"`
	}{publicationReceiptHeader: &header}
	if err := decodeStrictJSON(data, &wire); err != nil {
		return PublicationReceipt{}, publicationReceiptError("document", "requires a receipt with known JSON fields")
	}
	sources, err := decodePublicationSources(wire.Sources)
	if err != nil {
		return PublicationReceipt{}, err
	}
	header.Sources = sources
	if len(wire.Reviews) != 0 {
		header.Reviews, err = decodePublicationArray[evidence.ReviewCandidate](wire.Reviews, "reviews")
		if err != nil {
			return PublicationReceipt{}, err
		}
	}
	if len(wire.ReviewObservations) != 0 {
		header.ReviewObservations, err = decodePublicationArray[catalogs.SourceObservationLink](wire.ReviewObservations, "review_observations")
		if err != nil {
			return PublicationReceipt{}, err
		}
	}
	return PublicationReceipt(header), nil
}

func decodePublicationSources(data []byte) ([]PublicationSourceReceipt, error) {
	return decodePublicationArray[PublicationSourceReceipt](data, "sources")
}

func decodePublicationArray[T any](data []byte, field string) ([]T, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if token, err := decoder.Token(); err != nil || token != json.Delim('[') {
		return nil, publicationReceiptError(field, "requires an array")
	}
	var sources []T
	for decoder.More() {
		if len(sources) >= maxPublicationScopes {
			return nil, publicationReceiptError(field, "exceeds the receipt entry count limit")
		}
		var source T
		if err := decoder.Decode(&source); err != nil {
			return nil, publicationReceiptError(field, "requires entries with known JSON fields")
		}
		sources = append(sources, source)
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim(']') {
		return nil, publicationReceiptError(field, "requires a complete array")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, publicationReceiptError(field, "requires one array")
	}
	return sources, nil
}
