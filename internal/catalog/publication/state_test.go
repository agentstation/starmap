package publication

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
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

func TestRestoreStateSupersedesOlderSchemaCatalog(t *testing.T) {
	profile, run := admissionFixture(t)
	checkpoint := schemaCheckpointFixture(t, catalogs.RecognitionBillingSchemaVersion)
	restored, err := RestoreState(t.Context(), checkpoint.Data, checkpoint.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	supersession, found := restored.SchemaSupersession()
	want := SchemaSupersession{AcceptedSchemaVersion: catalogs.RecognitionBillingSchemaVersion, CurrentSchemaVersion: catalogs.CurrentCatalogSchemaVersion}
	if !found || supersession != want {
		t.Fatalf("supersession = %+v, %v; want %+v", supersession, found, want)
	}
	if restored.Generation().Manifest.SchemaVersion != catalogs.RecognitionBillingSchemaVersion {
		t.Fatal("restore replaced the accepted catalog")
	}
	if len(restored.history) != 1 || restored.history[0].ID != run.Attempts[0].Observation.ID {
		t.Fatal("restore changed the retained inputs")
	}
	successor, err := preparePublication(t.Context(), restored, profile, run, "successor-run")
	if err != nil {
		t.Fatal(err)
	}
	if successor.ReusedArtifact || successor.Next.Generation().Manifest.SchemaVersion != catalogs.CurrentCatalogSchemaVersion {
		t.Fatal("successor kept the older-schema artifact")
	}
	if _, found := successor.Next.SchemaSupersession(); found {
		t.Fatal("successor state carried the restore supersession")
	}
	next, err := EncodeState(successor.Next)
	if err != nil {
		t.Fatal(err)
	}
	again, err := RestoreState(t.Context(), next.Data, next.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := again.SchemaSupersession(); found {
		t.Fatal("current-schema checkpoint reported a supersession")
	}
}

func TestRestoreStateRejectsCurrentSchemaReplayMismatch(t *testing.T) {
	state := preparedStateFixture(t)
	baseline := state.baseline.Copy()
	bundle, err := artifact.Build(baseline)
	if err != nil {
		t.Fatal(err)
	}
	state.current, state.bundle = baseline, &bundle
	checkpoint, err := EncodeState(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreState(t.Context(), checkpoint.Data, checkpoint.Checksum)
	requireStateField(t, restored, err, "publication_admission.state.catalog")
}

func TestRestoreStateKeepsCheckpointBindingForOlderSchema(t *testing.T) {
	checkpoint := schemaCheckpointFixture(t, catalogs.RecognitionBillingSchemaVersion)
	for _, mutation := range []string{"checksum", "canonical", "archive", "statement"} {
		t.Run(mutation, func(t *testing.T) {
			data, expected := bytes.Clone(checkpoint.Data), checkpoint.Checksum
			field := "publication_admission.state"
			switch mutation {
			case "checksum":
				expected = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
				field = "publication_admission.state.checksum"
			case "canonical":
				var indented bytes.Buffer
				if err := json.Indent(&indented, data, "", " "); err != nil {
					t.Fatal(err)
				}
				data, expected = indented.Bytes(), digestForStateTest(indented.Bytes())
			case "archive", "statement":
				var record publicationStateRecord
				if err := json.Unmarshal(data, &record); err != nil {
					t.Fatal(err)
				}
				target := &record.Archive
				if mutation == "statement" {
					target = &record.Statement
				}
				*target = bytes.Clone(*target)
				(*target)[len(*target)/2] ^= 0xff
				var err error
				data, err = json.Marshal(record)
				if err != nil {
					t.Fatal(err)
				}
				expected, field = digestForStateTest(data), ""
			}
			restored, err := RestoreState(t.Context(), data, expected)
			requireStateField(t, restored, err, field)
		})
	}
}

// Artifact validation rejects a newer-schema payload before replay, so no checkpoint fixture can carry one.
// This test proves that the restore rule also refuses it.
func TestRestoreStateRejectsNewerSchemaCatalog(t *testing.T) {
	state := preparedStateFixture(t)
	state.current.Manifest.SchemaVersion = catalogs.CurrentCatalogSchemaVersion + 1
	err := verifyRestoredState(t.Context(), state)
	requireStateField(t, nil, err, "publication_admission.state.catalog.schema_version")
	if _, found := state.SchemaSupersession(); found {
		t.Fatal("a newer-schema catalog reported a supersession")
	}
}

// requireStateField checks a rejected restore. An empty field accepts any error from a lower layer.
func requireStateField(t *testing.T, restored *State, err error, field string) {
	t.Helper()
	if err == nil || restored != nil {
		t.Fatal("invalid checkpoint was accepted")
	}
	if field == "" {
		return
	}
	var validation *pkgerrors.ValidationError
	if !stderrors.As(err, &validation) || validation.Field != field {
		t.Fatalf("restore error = %v; want field %s", err, field)
	}
}

// preparedStateFixture returns an accepted current-schema state with one retained input.
func preparedStateFixture(t *testing.T) *State {
	t.Helper()
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
	return prepared.Next
}

// schemaCheckpointFixture re-declares the accepted catalog at an older schema, as an older publisher wrote it.
func schemaCheckpointFixture(t *testing.T, version uint64) ReceiptRecord {
	t.Helper()
	state := preparedStateFixture(t)
	accepted := state.current.Copy()
	current := fmt.Sprintf(`"schema_version":%d`, catalogs.CurrentCatalogSchemaVersion)
	if !bytes.Contains(accepted.Payload, []byte(current)) {
		t.Fatal("accepted payload does not declare the current schema")
	}
	accepted.Payload = bytes.Replace(accepted.Payload, []byte(current), []byte(fmt.Sprintf(`"schema_version":%d`, version)), 1)
	accepted.Manifest.SchemaVersion = version
	accepted.Manifest.ConsumerCompatibility = catalogs.ConsumerCompatibility{MinSchemaVersion: version, MaxSchemaVersion: version}
	accepted.Manifest.Payload = catalogs.DescribeCatalogPayload(accepted.Payload)
	bundle, err := artifact.Build(accepted)
	if err != nil {
		t.Fatal(err)
	}
	state.current, state.bundle = accepted, &bundle
	checkpoint, err := EncodeState(state)
	if err != nil {
		t.Fatal(err)
	}
	return checkpoint
}
