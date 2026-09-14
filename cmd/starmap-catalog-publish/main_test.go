package main

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/internal/catalog/publication"
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestPublicationCommandRetryAfterStagingFailure(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"one","object":"model"}]}`))
	}))
	defer server.Close()
	t.Setenv("PUBLICATION_FIXTURE_KEY", "fixture")
	root := t.TempDir()
	profile, state, checksum := commandInputs(t, root, server.URL)
	destination := filepath.Join(root, "outputs")
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(destination, "artifacts")
	if err := os.WriteFile(blocker, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-profile", profile, "-state", state, "-state-checksum", checksum,
		"-publisher-id", "command-publisher", "-run-id", "retry", "-output-dir", destination, "-workspace", filepath.Join(root, "workspace")}
	var output bytes.Buffer
	if err := run(t.Context(), args, &output); err == nil || output.Len() != 0 || requests.Load() != 1 {
		t.Fatal("staging failure changed acquisition or reported success", err)
	}
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	result := commandResult(t, args)
	if requests.Load() != 1 {
		t.Fatal("staging retry acquired different input")
	}
	commandReceipt(t, result)
	if err := os.WriteFile(profile, []byte("# changed request\n"+mustRead(t, profile)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), args, &output); err == nil || requests.Load() != 1 {
		t.Fatal("run identity accepted a different request or acquired again")
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPublicationCommandHelpAndCancellation(t *testing.T) {
	var output bytes.Buffer
	if err := run(t.Context(), []string{"-help"}, &output); !stderrors.Is(err, flag.ErrHelp) || !strings.Contains(output.String(), "-state-checksum") {
		t.Fatal("command did not explain its checkpoint trust input", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := prepare(ctx, prepareOptions{profile: "must-not-read"}); !stderrors.Is(err, context.Canceled) {
		t.Fatal("canceled command read files", err)
	}
}

func TestPublicationCommandCollectsStagesRestartsAndRetries(t *testing.T) {
	const acquisitionKey = "test-only-public-catalog-acquisition-secret"
	var requests atomic.Int32
	var failed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+acquisitionKey {
			t.Error("provider request missed the configured acquisition credential")
		}
		w.Header().Set("Content-Type", "application/json")
		if failed.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"fixture unavailable"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"one","object":"model"}]}`))
	}))
	defer server.Close()
	t.Setenv("PUBLICATION_FIXTURE_KEY", acquisitionKey)
	root := t.TempDir()
	profilePath, statePath, stateChecksum := commandInputs(t, root, server.URL)
	destination := filepath.Join(root, "prepared")
	arguments := []string{"-profile", profilePath, "-state", statePath, "-state-checksum", stateChecksum, "-publisher-id", "command-publisher", "-output-dir", destination, "-run-id", "one", "-workspace", filepath.Join(root, "workspace")}
	first := commandResult(t, arguments)
	if first.Status != "prepared" || requests.Load() != 1 {
		t.Fatalf("status=%s requests=%d", first.Status, requests.Load())
	}
	for _, path := range []string{first.StatePath, first.ReceiptPath} {
		if strings.Contains(mustRead(t, path), acquisitionKey) {
			t.Fatal("publication output exposed the acquisition credential")
		}
	}
	receipt := commandReceipt(t, first)
	if !receipt.FreshAcquisition {
		t.Fatal("successful request did not produce fresh evidence")
	}
	retry := commandResult(t, arguments)
	if retry != first || requests.Load() != 1 {
		t.Fatal("retry acquired new evidence or changed prepared output")
	}
	failed.Store(true)
	secondArguments := append([]string(nil), arguments...)
	for i := range secondArguments {
		switch secondArguments[i] {
		case "-state":
			secondArguments[i+1] = first.StatePath
		case "-state-checksum":
			secondArguments[i+1] = first.StateChecksum
		case "-run-id":
			secondArguments[i+1] = "two"
		}
	}
	second := commandResult(t, secondArguments)
	secondReceipt := commandReceipt(t, second)
	if !second.ReusedArtifact || second.ArchiveChecksum != first.ArchiveChecksum || secondReceipt.FreshAcquisition || requests.Load() != 2 {
		t.Fatal("restart failed to retain the accepted catalog after a provider error")
	}
	if secondReceipt.Sources[0].Observation.ObservationID != receipt.Sources[0].Observation.ObservationID {
		t.Fatal("restart changed retained observation identity")
	}
	rejected := append([]string(nil), arguments...)
	for i := range rejected {
		if rejected[i] == "-run-id" {
			rejected[i+1] = "rejected"
		}
	}
	var output bytes.Buffer
	if err := run(t.Context(), rejected, &output); err == nil || output.Len() != 0 {
		t.Fatal("first acquisition failure produced a success report")
	}
	rejections, err := filepath.Glob(filepath.Join(destination, "rejected-*.json"))
	if err != nil || len(rejections) != 1 {
		t.Fatal("rejected acquisition did not preserve its diagnostic record", err)
	}
	var diagnostic struct {
		Decision publication.Decision `json:"decision"`
	}
	if err := json.Unmarshal([]byte(mustRead(t, rejections[0])), &diagnostic); err != nil {
		t.Fatal(err)
	}
	if diagnostic.Decision.Allowed || len(diagnostic.Decision.Inputs) != 0 || diagnostic.Decision.Scopes[0].Attempt != publication.Failed {
		t.Fatal("rejection record lost the source failure or retained publishable inputs")
	}
	for i := range arguments {
		if arguments[i] == "-publisher-id" {
			arguments[i+1] = "different-publisher"
		}
	}
	if err := run(t.Context(), arguments, &output); err == nil {
		t.Fatal("checkpoint allowed a different publisher identity")
	}
}

func commandResult(t *testing.T, args []string) prepareReport {
	t.Helper()
	var output bytes.Buffer
	if err := run(t.Context(), args, &output); err != nil {
		t.Fatal(err)
	}
	var report prepareReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	return report
}

func commandReceipt(t *testing.T, report prepareReport) artifact.PublicationReceipt {
	t.Helper()
	archive, err := os.ReadFile(filepath.Join(report.ArtifactDirectory, artifact.Filename))
	if err != nil {
		t.Fatal(err)
	}
	statement, err := os.ReadFile(filepath.Join(report.ArtifactDirectory, artifact.AttestationFilename))
	if err != nil {
		t.Fatal(err)
	}
	generation, err := artifact.Open(archive, statement)
	if err != nil {
		t.Fatal(err)
	}
	semantic, err := generation.SemanticChecksum()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(report.ReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := artifact.VerifyPublicationReceipt(raw, report.ReceiptChecksum, artifact.PublicationArtifact{GenerationID: report.GenerationID, CatalogChecksum: semantic, PayloadChecksum: generation.Manifest.Payload.Checksum, ArchiveChecksum: report.ArchiveChecksum})
	if err != nil {
		t.Fatal(err)
	}
	state, err := os.ReadFile(report.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publication.RestoreState(t.Context(), state, report.StateChecksum); err != nil {
		t.Fatal(err)
	}
	return receipt
}

func commandInputs(t *testing.T, root, endpoint string) (string, string, string) {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetAuthor(catalogs.Author{ID: "author", Name: "Author"}); err != nil {
		t.Fatal(err)
	}
	if err := builder.SetAuthorModel("author", catalogs.Model{ID: "one", Name: "One", Authors: []catalogs.Author{{ID: "author", Name: "Author"}}}); err != nil {
		t.Fatal(err)
	}
	credentials := testcatalog.APIKeyCredentials("PUBLICATION_FIXTURE_KEY", "Authorization", catalogs.ProviderCredentialSchemeBearer)
	provider := catalogs.Provider{ID: "provider", Name: "Provider", Models: map[string]*catalogs.Model{"one": {ID: "one", Name: "One", ModelRef: "author/one"}}, Credentials: credentials, Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{Type: catalogs.EndpointTypeOpenAI, URL: endpoint, ProtocolOptions: testcatalog.OpenAIProtocolOptions()}}}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../pkg/catalogs/testdata/generation/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := catalogs.ParseGenerationManifestJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Payload = catalogs.DescribeCatalogPayload(payload)
	manifest.SchemaVersion = catalogs.CatalogPayloadSchemaVersion(catalog)
	manifest.ConsumerCompatibility = catalogs.ConsumerCompatibility{MinSchemaVersion: manifest.SchemaVersion, MaxSchemaVersion: manifest.SchemaVersion}
	state, err := publication.NewState(catalogs.Generation{Manifest: manifest, Payload: payload}, "command-publisher")
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := publication.EncodeState(state)
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(root, "initial-state.json")
	if err := os.WriteFile(statePath, checkpoint.Data, 0o600); err != nil {
		t.Fatal(err)
	}
	binding := sources.ProviderAcquisitionBinding{SchemaVersion: sources.ProviderAcquisitionBindingSchemaVersion, ID: "account", Revision: "1", ProviderID: "provider", AccountID: "fixture-account", Region: "global", APISurface: "models", CredentialRole: sources.ProviderBindingCatalogAcquisition, CredentialProfileID: credentials.CatalogAcquisition.Alternatives[0], MembershipAuthority: sources.ProviderMembershipScope}
	profile, err := yaml.Marshal(map[string]any{"policy_version": "fixture-v1", "scopes": []any{map[string]any{"source": sources.ProvidersID, "binding": binding, "required": true, "enabled": true, "allow_missing": false, "max_retained_age": "4h", "disabled_action": "preserve"}}})
	if err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(root, "publication.yaml")
	if err := os.WriteFile(profilePath, profile, 0o600); err != nil {
		t.Fatal(err)
	}
	return profilePath, statePath, checkpoint.Checksum
}
