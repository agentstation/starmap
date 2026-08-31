package payload

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/agentstation/starmap/pkg/errors"
)

// RecordIssue describes one malformed member of an otherwise valid collection.
// Subject is a stable collection-relative identity and never contains raw input.
type RecordIssue struct {
	// Subject identifies the record within its collection.
	Subject string
	// Err is the typed decode or validation failure.
	Err error
}

// RecordReport describes bounded per-record decoding.
type RecordReport struct {
	// Accepted is the number of successfully decoded records.
	Accepted int
	// Rejected includes malformed and excess records.
	Rejected int
	// Issues contains bounded details for malformed records.
	Issues []RecordIssue
	// Truncated reports that excess records were not decoded.
	Truncated bool
}

// QuarantineError reports a partial result with usable records and quarantined
// malformed or excess siblings.
type QuarantineError struct {
	// Collection identifies the affected source collection.
	Collection string
	// Report describes the usable and quarantined records.
	Report RecordReport
}

// Error implements error.
func (e *QuarantineError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: quarantined %d record(s)", e.Collection, e.Report.Rejected)
}

// Unwrap exposes the first record error for standard errors.Is/errors.As use.
func (e *QuarantineError) Unwrap() error {
	if e == nil || len(e.Report.Issues) == 0 {
		return nil
	}
	return e.Report.Issues[0].Err
}

// Err returns a typed partial-result error when the report contains quarantined records.
func (r RecordReport) Err(collection string) error {
	if r.Rejected == 0 {
		return nil
	}
	return &QuarantineError{Collection: collection, Report: r}
}

// DecodeJSONArray requires a valid JSON array envelope, then decodes each member
// independently. It decodes at most max records.
func DecodeJSONArray[T any](data json.RawMessage, collection string, limit int) ([]T, RecordReport, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, RecordReport{}, errors.WrapParse("json", collection, err)
	}
	if raw == nil {
		return nil, RecordReport{}, &errors.ValidationError{
			Field: collection, Message: "required array is missing or null",
		}
	}

	report := RecordReport{}
	if limit >= 0 && len(raw) > limit {
		report.Rejected += len(raw) - limit
		report.Truncated = true
		raw = raw[:limit]
	}
	decoded := make([]T, 0, len(raw))
	for index, record := range raw {
		var value T
		if err := json.Unmarshal(record, &value); err != nil {
			report.Rejected++
			report.Issues = append(report.Issues, RecordIssue{
				Subject: fmt.Sprintf("%s[%d]", collection, index),
				Err:     errors.WrapParse("json", fmt.Sprintf("%s[%d]", collection, index), err),
			})
			continue
		}
		decoded = append(decoded, value)
		report.Accepted++
	}
	return decoded, report, nil
}

// DecodeJSONObject requires a valid JSON object envelope, then decodes values
// independently in sorted key order. It decodes at most max records.
func DecodeJSONObject[T any](data json.RawMessage, collection string, limit int) (map[string]T, RecordReport, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, RecordReport{}, errors.WrapParse("json", collection, err)
	}
	if raw == nil {
		return nil, RecordReport{}, &errors.ValidationError{
			Field: collection, Message: "required object is missing or null",
		}
	}

	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	report := RecordReport{}
	if limit >= 0 && len(keys) > limit {
		report.Rejected += len(keys) - limit
		report.Truncated = true
		keys = keys[:limit]
	}
	decoded := make(map[string]T, len(keys))
	for _, key := range keys {
		var value T
		if err := json.Unmarshal(raw[key], &value); err != nil {
			report.Rejected++
			report.Issues = append(report.Issues, RecordIssue{
				Subject: collection + "[" + key + "]",
				Err:     errors.WrapParse("json", collection+"["+key+"]", err),
			})
			continue
		}
		decoded[key] = value
		report.Accepted++
	}
	return decoded, report, nil
}
