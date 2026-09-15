package publication

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestPublicationStateRoundTripKeepsOriginalEvidenceAndArtifact(t *testing.T) {
	profile, run := admissionFixture(t)
	empty, err := catalogs.NewEmpty().Build()
	if err != nil {
		t.Fatal(err)
	}
	original, err := NewState(publicationBaselineFixture(t, empty), "checkpoint-publisher")
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := preparePublication(t.Context(), original, profile, run, "checkpoint-run")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeState(prepared.Next)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreState(t.Context(), encoded.Data, encoded.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	again, err := EncodeState(restored)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded.Data, again.Data) || len(restored.history) != 1 || restored.history[0].ID != run.Attempts[0].Observation.ID {
		t.Fatal("checkpoint changed original evidence")
	}
	if !bytes.Equal(prepared.Bundle.Data, restored.bundle.Data) {
		t.Fatal("checkpoint changed accepted artifact")
	}
	for _, mutation := range []string{"checksum", "payload", "binding", "case", "duplicate", "trailing"} {
		t.Run(mutation, func(t *testing.T) {
			data := bytes.Clone(encoded.Data)
			expected := encoded.Checksum
			switch mutation {
			case "checksum":
				expected = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
			case "payload", "binding":
				var record publicationStateRecord
				if err := json.Unmarshal(data, &record); err != nil {
					t.Fatal(err)
				}
				if mutation == "payload" {
					record.Inputs[0].Payload = []byte("{}")
				} else {
					record.Inputs[0].Receipt.ProviderBinding.AccountID = "other-account"
				}
				data, err = json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				expected = digestForStateTest(data)
			case "case":
				data = bytes.Replace(data, []byte(`"publisher_id"`), []byte(`"Publisher_id"`), 1)
				expected = digestForStateTest(data)
			case "duplicate":
				data = bytes.Replace(data, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1)
				expected = digestForStateTest(data)
			case "trailing":
				data = append(data, []byte("{}")...)
				expected = digestForStateTest(data)
			}
			result, err := RestoreState(t.Context(), data, expected)
			if err == nil || result != nil {
				t.Fatal("invalid checkpoint was accepted")
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if result, err := RestoreState(ctx, encoded.Data, encoded.Checksum); err == nil || result != nil {
		t.Fatal("canceled checkpoint restored")
	}
}

func digestForStateTest(data []byte) string { return receiptChecksum(data) }
