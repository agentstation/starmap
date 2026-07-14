package catalogremote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/constants"
)

func TestRemoteClientDefaultHTTPTimeout(t *testing.T) {
	client, err := NewClient("https://starmap.example.com/api/v1", nil, catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.httpClient == http.DefaultClient || client.httpClient.Timeout != constants.DefaultHTTPTimeout {
		t.Fatalf("default HTTP client = %#v, want isolated timeout %s", client.httpClient, constants.DefaultHTTPTimeout)
	}
}

func TestRemoteClientRejectsCrossOriginRedirect(t *testing.T) {
	var redirectedRequests atomic.Int32
	redirected := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirectedRequests.Add(1)
	}))
	defer redirected.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", redirected.URL)
		writer.WriteHeader(http.StatusFound)
	}))
	defer origin.Close()

	client, err := NewClient(origin.URL, origin.Client(), catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.FetchCurrent(context.Background()); err == nil {
		t.Fatal("FetchCurrent followed a cross-origin redirect")
	}
	if got := redirectedRequests.Load(); got != 0 {
		t.Fatalf("cross-origin requests = %d, want 0", got)
	}
}

func TestRemoteCatalogFetchValidatesManifestSnapshotChecksumAndExactSchema(t *testing.T) {
	current := catalogs.CurrentCatalogSchemaVersion
	valid := remoteTestGeneration(t, current)
	for _, test := range []struct {
		name            string
		generation      catalogstore.Generation
		mutateSnapshot  func([]byte) []byte
		manifestType    string
		snapshotType    string
		wantError       bool
		wantSnapshotGet bool
	}{
		{name: "valid", generation: valid, manifestType: ManifestMediaType, snapshotType: catalogs.CatalogPayloadMediaType, wantSnapshotGet: true},
		{name: "corrupt snapshot", generation: valid, mutateSnapshot: func(data []byte) []byte {
			copyData := append([]byte(nil), data...)
			copyData[len(copyData)-1] ^= 1
			return copyData
		}, manifestType: ManifestMediaType, snapshotType: catalogs.CatalogPayloadMediaType, wantError: true, wantSnapshotGet: true},
		{name: "wrong manifest media type", generation: valid, manifestType: "application/json", snapshotType: catalogs.CatalogPayloadMediaType, wantError: true},
		{name: "wrong snapshot media type", generation: valid, manifestType: ManifestMediaType, snapshotType: "application/json", wantError: true, wantSnapshotGet: true},
		{name: "wrong schema before snapshot", generation: remoteTestGeneration(t, 1), manifestType: ManifestMediaType, snapshotType: catalogs.CatalogPayloadMediaType, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifest, err := remoteTestManifestBytes(test.generation.Manifest)
			if err != nil {
				t.Fatalf("MarshalManifest: %v", err)
			}
			var snapshotGets atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case ManifestPath:
					writer.Header().Set("Content-Type", test.manifestType)
					_, _ = writer.Write(manifest)
				case SnapshotPath(test.generation.Manifest.GenerationID):
					snapshotGets.Add(1)
					writer.Header().Set("Content-Type", test.snapshotType)
					payload := test.generation.Payload
					if test.mutateSnapshot != nil {
						payload = test.mutateSnapshot(payload)
					}
					_, _ = writer.Write(payload)
				default:
					http.NotFound(writer, request)
				}
			}))
			defer server.Close()
			client, err := NewClient(server.URL, server.Client(), catalogs.CurrentCatalogSchemaVersion)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			got, err := client.FetchCurrent(context.Background())
			if (err != nil) != test.wantError {
				t.Fatalf("FetchCurrent = %#v/%v", got, err)
			}
			if (snapshotGets.Load() > 0) != test.wantSnapshotGet {
				t.Fatalf("snapshot GETs = %d, want requested=%t", snapshotGets.Load(), test.wantSnapshotGet)
			}
			if err == nil && got.Manifest.GenerationID != valid.Manifest.GenerationID {
				t.Fatalf("generation ID = %q", got.Manifest.GenerationID)
			}
		})
	}
}

func TestRemoteCatalogRejectsCredentialScopedManifestBeforeSnapshotRequest(t *testing.T) {
	generation := remoteTestGeneration(t, catalogs.CurrentCatalogSchemaVersion)
	generation.Manifest.SourceObservations[0].Metrics.Scope = catalogmeta.ObservationScopeCredentialScoped
	manifest, err := json.Marshal(generation.Manifest)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var snapshotGets atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case ManifestPath:
			writer.Header().Set("Content-Type", ManifestMediaType)
			_, _ = writer.Write(manifest)
		case SnapshotPath(generation.Manifest.GenerationID):
			snapshotGets.Add(1)
			writer.Header().Set("Content-Type", catalogs.CatalogPayloadMediaType)
			_, _ = writer.Write(generation.Payload)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client, err := NewClient(server.URL, server.Client(), catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.FetchCurrent(context.Background()); err == nil {
		t.Fatal("FetchCurrent accepted credential-scoped manifest")
	}
	if got := snapshotGets.Load(); got != 0 {
		t.Fatalf("snapshot GETs = %d, want 0", got)
	}
}

func remoteTestManifestBytes(manifest catalogs.GenerationManifest) ([]byte, error) {
	wantSchema := manifest.SchemaVersion
	manifest.SchemaVersion = catalogs.CurrentCatalogSchemaVersion
	data, err := MarshalManifest(manifest)
	if err != nil || wantSchema == catalogs.CurrentCatalogSchemaVersion {
		return data, err
	}
	return bytes.Replace(data, []byte(`"schema_version":2`), []byte(fmt.Sprintf(`"schema_version":%d`, wantSchema)), 1), nil
}

func remoteTestGeneration(t *testing.T, schemaVersion uint64) catalogstore.Generation {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "remote-test", Name: "Remote Test"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	payload, err := catalogstore.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	descriptor := catalogs.DescribeCatalogPayload(payload)
	generatedAt := time.Date(2026, time.July, 11, 3, 0, 0, 0, time.UTC)
	generation := catalogstore.Generation{
		Manifest: catalogs.GenerationManifest{
			ManifestVersion: catalogs.CurrentGenerationManifestVersion,
			SchemaVersion:   schemaVersion, GenerationID: fmt.Sprintf("remote-generation-%d", schemaVersion),
			GeneratedAt: generatedAt, Payload: descriptor,
			Validation: catalogs.GenerationValidationReport{
				ValidatorVersion: "remote-test/v1", ValidatedAt: generatedAt, Status: catalogs.GenerationValidationPassed,
				Checks: []catalogs.GenerationValidationCheck{{Name: "test", Status: catalogs.GenerationValidationCheckPassed}},
			},
			SyncRunID: "remote-sync-run",
			SourceObservations: []catalogs.SourceObservationLink{{
				Source: catalogmeta.LocalCatalogID, ObservationID: "remote-observation", ObservedAt: generatedAt,
				Revision:     catalogmeta.ObservationRevision{Kind: catalogmeta.ObservationRevisionKindContentDigest, Value: descriptor.Checksum},
				Completeness: catalogmeta.ObservationCompletenessComplete, Status: catalogmeta.ObservationStatusSucceeded,
				EvidenceChecksum: descriptor.Checksum,
			}},
			Completeness: catalogs.GenerationCompletenessComplete,
		},
		Payload: payload,
	}
	if schemaVersion == catalogs.CurrentCatalogSchemaVersion {
		if err := generation.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	}
	return generation
}
