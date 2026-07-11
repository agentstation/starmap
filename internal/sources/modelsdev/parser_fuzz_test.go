package modelsdev

import (
	"testing"

	"github.com/agentstation/starmap/pkg/constants"
)

func FuzzParseAPIDataNoPanic(f *testing.F) {
	f.Add([]byte(`{"provider":{"id":"provider","name":"Provider","models":{}}}`))
	f.Add([]byte(`{"provider":{"models":null}}`))
	f.Add([]byte(`{"provider":{"models":[]}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > constants.MaxSourcePayloadBytes+1 {
			t.Skip()
		}
		_, _ = parseAPIData(data)
	})
}
