package provenance

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/goccy/go-yaml"
)

func TestBindingProvenanceRoundTripsWithoutChangingLegacyShape(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		entry := Entry{Source: evidence.ProvidersID, ObservationID: "observation", Timestamp: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), Value: 1000}
		if scoped {
			entry.ProviderBindingID = "one"
			entry.ProviderBindingRevision = "1"
		}
		encoded, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		if !scoped && bytes.Contains(encoded, []byte("ProviderBinding")) {
			t.Fatal("legacy provenance gained empty binding keys")
		}
		for _, transport := range []string{"json", "yaml"} {
			t.Run(transport+map[bool]string{false: "-legacy", true: "-scoped"}[scoped], func(t *testing.T) {
				var restored Entry
				data := encoded
				if transport == "yaml" {
					data, err = yaml.Marshal(entry)
					if err != nil {
						t.Fatal(err)
					}
					err = yaml.Unmarshal(data, &restored)
				} else {
					err = json.Unmarshal(data, &restored)
				}
				if err != nil {
					t.Fatal(err)
				}
				if restored.ProviderBindingID != entry.ProviderBindingID || restored.ProviderBindingRevision != entry.ProviderBindingRevision {
					t.Fatal("binding provenance was lost")
				}
				again, err := json.Marshal(restored)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(again, encoded) {
					t.Fatalf("canonical bytes changed through %s", transport)
				}
			})
		}
	}
}
