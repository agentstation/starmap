package starmap

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogremote"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogstore"
	"github.com/agentstation/starmap/pkg/constants"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestNewPrefersDurableCurrentOverCorruptLocalCompatibilityView(t *testing.T) {
	store := catalogstore.NewMemory()
	generation := rootRemoteGeneration(t)
	if err := store.Commit(context.Background(), generation, ""); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	localPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(localPath, "providers.yaml"), []byte("providers: [unterminated\n"), constants.SecureFilePermissions); err != nil {
		t.Fatalf("Write corrupt local view: %v", err)
	}

	client, err := New(WithCatalogStore(store), WithCatalogExportPath(localPath))
	if err != nil {
		t.Fatalf("New rejected valid durable current because local view was corrupt: %v", err)
	}
	if got := client.CurrentGenerationID(); got != generation.Manifest.GenerationID {
		t.Fatalf("generation ID = %q, want %q", got, generation.Manifest.GenerationID)
	}
	if _, err := client.Catalog().Provider("remote-root"); err != nil {
		t.Fatalf("durable catalog was not published: %v", err)
	}
}

func TestRemoteCatalogCorruptOrIncompatibleGenerationCannotReplaceCurrent(t *testing.T) {
	valid := rootRemoteGeneration(t)
	for _, test := range []struct {
		name         string
		generation   catalogstore.Generation
		corrupt      bool
		wantSnapshot bool
	}{
		{name: "corrupt payload", generation: valid, corrupt: true, wantSnapshot: true},
		{name: "incompatible manifest", generation: incompatibleRemoteGeneration(t, valid)},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifest, err := rootRemoteManifestBytes(test.generation.Manifest)
			if err != nil {
				t.Fatalf("MarshalManifest: %v", err)
			}
			var snapshotRequests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case catalogremote.ManifestPath:
					writer.Header().Set("Content-Type", catalogremote.ManifestMediaType)
					_, _ = writer.Write(manifest)
				case catalogremote.SnapshotPath(test.generation.Manifest.GenerationID):
					snapshotRequests.Add(1)
					writer.Header().Set("Content-Type", catalogs.CatalogPayloadMediaType)
					payload := append([]byte(nil), test.generation.Payload...)
					if test.corrupt {
						payload[len(payload)-1] ^= 1
					}
					_, _ = writer.Write(payload)
				default:
					http.NotFound(writer, request)
				}
			}))
			defer server.Close()

			store := catalogstore.NewMemory()
			client, err := New(WithCatalogStore(store), WithRemoteServerOnly(server.URL))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			beforeCatalog := client.Catalog()
			beforeID := client.CurrentGenerationID()
			if err := client.Update(context.Background()); err == nil {
				t.Fatal("invalid remote generation replaced current catalog")
			}
			if client.Catalog() != beforeCatalog || client.CurrentGenerationID() != beforeID {
				t.Fatalf("current catalog changed to %q", client.CurrentGenerationID())
			}
			if _, err := store.Current(context.Background()); !pkgerrors.IsNotFound(err) {
				t.Fatalf("invalid remote generation reached durable store: %v", err)
			}
			if (snapshotRequests.Load() > 0) != test.wantSnapshot {
				t.Fatalf("snapshot requests = %d, want requested=%t", snapshotRequests.Load(), test.wantSnapshot)
			}
		})
	}
}

func rootRemoteManifestBytes(manifest catalogs.GenerationManifest) ([]byte, error) {
	wantSchema := manifest.SchemaVersion
	manifest.SchemaVersion = catalogs.CurrentCatalogSchemaVersion
	data, err := catalogremote.MarshalManifest(manifest)
	if err != nil || wantSchema == catalogs.CurrentCatalogSchemaVersion {
		return data, err
	}
	return bytes.Replace(data, []byte(`"schema_version":2`), []byte(fmt.Sprintf(`"schema_version":%d`, wantSchema)), 1), nil
}

func rootRemoteGeneration(t *testing.T) catalogstore.Generation {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "remote-root", Name: "Remote Root"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	at := time.Date(2026, time.July, 11, 4, 0, 0, 0, time.UTC)
	observation, err := sources.NewObservation(sources.LocalCatalogID, catalog, sources.ObservationMetadata{
		ObservedAt: at, Revision: sources.Revision{Kind: sources.RevisionKindContentDigest},
		Completeness: sources.ObservationCompletenessComplete, Status: sources.ObservationStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("NewObservation: %v", err)
	}
	generation, err := generationTestClient(at).newGeneration(catalog, []sources.Observation{observation})
	if err != nil {
		t.Fatalf("newGeneration: %v", err)
	}
	return generation
}

func incompatibleRemoteGeneration(t *testing.T, generation catalogstore.Generation) catalogstore.Generation {
	t.Helper()
	incompatible := generation.Copy()
	incompatible.Manifest.SchemaVersion = 1
	return incompatible
}
