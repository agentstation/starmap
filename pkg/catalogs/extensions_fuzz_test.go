package catalogs

import (
	"encoding/json"
	"testing"

	"github.com/agentstation/starmap/pkg/constants"
)

func FuzzSourceExtensionNoPanic(f *testing.F) {
	f.Add([]byte(`{"fields":{"nested":{"enabled":true},"values":[1,2,3]}}`))
	f.Add([]byte(`{"fields":null}`))
	f.Add([]byte(`{"fields":{"number":1e100}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > constants.MaxSourcePayloadBytes {
			t.Skip()
		}
		var extension SourceExtension
		if err := json.Unmarshal(data, &extension); err != nil {
			return
		}
		copied := extension.Copy()
		normalized := NormalizeExtensionFields(copied.Fields)
		_, _ = json.Marshal(SourceExtension{Fields: normalized})
	})
}
