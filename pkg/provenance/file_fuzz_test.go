package provenance

import (
	"testing"

	"github.com/agentstation/starmap/pkg/constants"
	"github.com/goccy/go-yaml"
)

func FuzzFileYAMLDecodeNoPanic(f *testing.F) {
	f.Add([]byte(`provenance:
  model:provider%2Fmodel:Name:
  - source: providers
    field: Name
    value: Model
    timestamp: 2026-07-28T18:00:00Z
`))
	f.Add([]byte("provenance: null\n"))
	f.Add([]byte("provenance:\n  malformed: [\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > constants.MaxSourcePayloadBytes {
			t.Skip()
		}
		var file File
		if err := yaml.Unmarshal(data, &file); err != nil {
			return
		}
		report := GenerateReport(file.Provenance)
		_ = report.String()
		_, _ = yaml.Marshal(file)
	})
}
