package catalogs

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestArchitectureLegacyEncodingRemainsExact(t *testing.T) {
	for _, body := range []string{
		`{"quantized":false,"fine_tuned":false}`,
		`{"parameter_count":"12B","type":"transformer","tokenizer":"gpt","quantization":"fp16","quantized":true,"fine_tuned":false,"base_model":"base"}`,
	} {
		t.Run(body, func(t *testing.T) {
			var architecture ModelArchitecture
			if err := json.Unmarshal([]byte(body), &architecture); err != nil {
				t.Fatal(err)
			}
			actual, err := json.Marshal(architecture)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actual, []byte(body)) {
				t.Errorf("legacy bytes changed: got %s, want %s", actual, body)
			}
		})
	}
}
