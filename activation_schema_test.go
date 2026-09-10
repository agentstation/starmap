package starmap

import (
	"encoding/json"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
)

func TestActivateChecksManifestAndPayloadSchemaTogether(t *testing.T) {
	for _, test := range []struct {
		name     string
		manifest uint64
		accept   bool
	}{
		{"legacy generation", 6, true},
		{"mismatched current manifest", catalogs.CurrentCatalogSchemaVersion, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			generation := rootRemoteGeneration(t)
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(generation.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			payload["schema_version"] = json.RawMessage("6")
			delete(payload, "membership_scopes")
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			generation.Payload = raw
			generation.Manifest.SchemaVersion = test.manifest
			generation.Manifest.ConsumerCompatibility = catalogs.ConsumerCompatibility{MinSchemaVersion: test.manifest, MaxSchemaVersion: test.manifest}
			generation.Manifest.Payload = catalogs.DescribeCatalogPayload(raw)
			store := storage.NewMemory()
			client, err := New(WithCatalogStore(store))
			if err != nil {
				t.Fatal(err)
			}
			before := client.CurrentCatalogState()
			_, err = client.Activate(t.Context(), generation)
			if (err == nil) != test.accept {
				t.Fatalf("accepted=%t, want %t: %v", err == nil, test.accept, err)
			}
			if !test.accept && client.CurrentCatalogState().GenerationID != before.GenerationID {
				t.Fatal("rejected schema changed active generation")
			}
			if test.accept {
				restarted, err := New(WithCatalogStore(store))
				if err != nil {
					t.Fatal(err)
				}
				if restarted.CurrentGenerationID() != generation.Manifest.GenerationID {
					t.Fatal("restart changed legacy generation identity")
				}
				derived, err := generationTestClient(generation.Manifest.GeneratedAt).newGeneration(restarted.Catalog(), CandidateEvidence{}, "")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := catalogs.DecodeCatalogGeneration(derived); err != nil {
					t.Fatalf("derived legacy generation: %v", err)
				}
			}

		})
	}
}
