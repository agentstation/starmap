package administration

import (
	"bytes"
	"encoding/json"

	"github.com/agentstation/starmap/pkg/errors"
)

// Reports holds detached configuration reports prepared by the host application.
// The host must redact secrets before construction. Reports require no catalog startup.
type Reports struct {
	schema    []byte
	effective []byte
}

// NewReports validates and copies versioned JSON objects for administrative diagnostics.
func NewReports(schema, effective []byte) (*Reports, error) {
	for _, data := range [][]byte{schema, effective} {
		var version struct {
			SchemaVersion int `json:"schema_version"`
		}
		if len(data) == 0 || len(data) > maximumIdentityRecordBytes || json.Unmarshal(data, &version) != nil || version.SchemaVersion != 1 {
			return nil, &errors.ValidationError{Field: "server.configuration_report", Message: "requires a bounded schema version 1 JSON object"}
		}
	}
	return &Reports{schema: bytes.Clone(schema), effective: bytes.Clone(effective)}, nil
}

// Schema returns a detached configuration descriptor report.
func (r *Reports) Schema() []byte { return bytes.Clone(r.schema) }

// Effective returns a detached report of redacted values, origins, and file paths.
func (r *Reports) Effective() []byte { return bytes.Clone(r.effective) }
