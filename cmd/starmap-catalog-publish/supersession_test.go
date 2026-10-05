package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
)

func TestPublicationCommandReportsOlderSchemaSupersession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"one","object":"model"}]}`))
	}))
	defer server.Close()
	t.Setenv("PUBLICATION_FIXTURE_KEY", "fixture")
	root := t.TempDir()
	profile, state, checksum := commandInputs(t, root, server.URL)
	prepare := func(state, checksum, runID string) (prepareReport, []byte) {
		t.Helper()
		var output bytes.Buffer
		args := []string{"-profile", profile, "-state", state, "-state-checksum", checksum,
			"-publisher-id", "command-publisher", "-run-id", runID, "-output-dir", filepath.Join(root, runID)}
		if err := run(t.Context(), args, &output); err != nil {
			t.Fatal(err)
		}
		var report prepareReport
		if err := json.Unmarshal(output.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		return report, output.Bytes()
	}
	current, raw := prepare(state, checksum, "current-schema")
	if current.SchemaSupersession != nil || bytes.Contains(raw, []byte(`"schema_supersession"`)) {
		t.Fatal("a current-schema checkpoint reported a supersession")
	}
	older, olderChecksum := olderSchemaCheckpoint(t, current.StatePath, filepath.Join(root, "older-state.json"))
	superseded, raw := prepare(older, olderChecksum, "older-schema")
	want := schemaSupersessionReport{AcceptedSchemaVersion: catalogs.RecognitionBillingSchemaVersion, CurrentSchemaVersion: catalogs.CurrentCatalogSchemaVersion}
	if superseded.SchemaSupersession == nil || *superseded.SchemaSupersession != want {
		t.Fatalf("supersession = %+v; want %+v", superseded.SchemaSupersession, want)
	}
	expected := fmt.Sprintf(`"schema_supersession":{"accepted_schema_version":%d,"current_schema_version":%d}`, want.AcceptedSchemaVersion, want.CurrentSchemaVersion)
	if !bytes.Contains(raw, []byte(expected)) {
		t.Fatalf("report %s does not contain %s", raw, expected)
	}
	commandReceipt(t, superseded)
	archive := mustRead(t, filepath.Join(superseded.ArtifactDirectory, artifact.Filename))
	statement := mustRead(t, filepath.Join(superseded.ArtifactDirectory, artifact.AttestationFilename))
	generation, err := artifact.Open([]byte(archive), []byte(statement))
	if err != nil {
		t.Fatal(err)
	}
	if superseded.ReusedArtifact || generation.Manifest.SchemaVersion != catalogs.CurrentCatalogSchemaVersion {
		t.Fatal("successor did not publish a current-schema catalog")
	}
	successor, raw := prepare(superseded.StatePath, superseded.StateChecksum, "successor")
	if successor.SchemaSupersession != nil || bytes.Contains(raw, []byte(`"schema_supersession"`)) {
		t.Fatal("the successor checkpoint reported a supersession")
	}
}

// olderSchemaCheckpoint re-declares the accepted catalog of a checkpoint at an older schema, as an older publisher wrote it.
func olderSchemaCheckpoint(t *testing.T, source, destination string) (string, string) {
	t.Helper()
	var record struct {
		Version     int             `json:"version"`
		PublisherID string          `json:"publisher_id"`
		Baseline    json.RawMessage `json:"baseline"`
		Archive     []byte          `json:"archive"`
		Statement   []byte          `json:"statement"`
		Inputs      json.RawMessage `json:"inputs"`
	}
	if err := json.Unmarshal([]byte(mustRead(t, source)), &record); err != nil {
		t.Fatal(err)
	}
	accepted, err := artifact.Open(record.Archive, record.Statement)
	if err != nil {
		t.Fatal(err)
	}
	version := catalogs.RecognitionBillingSchemaVersion
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
	record.Archive, record.Statement = bundle.Data, bundle.Attestation
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return destination, digest(data)
}
