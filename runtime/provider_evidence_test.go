package runtime

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestProviderEvidenceRefusesInvalidBatchBeforeRetention(t *testing.T) {
	for _, durable := range []bool{false, true} {
		t.Run(map[bool]string{false: "memory", true: "durable"}[durable], func(t *testing.T) {
			for _, defect := range []string{"digest", "payload-provider", "invalid-payload", "missing-time", "provider-alias", "multiple-providers"} {
				t.Run(defect, func(t *testing.T) {
					root := ""
					if durable {
						root = t.TempDir()
					}
					store, err := newLayerStore(root)
					if err != nil {
						t.Fatal(err)
					}
					r := &Runtime{store: store}
					at := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
					old := testProviderLayer(t, "first", "old", "Old", at)
					if err := r.retainProviders(t.Context(), []ProviderLayer{old}); err != nil {
						t.Fatal(err)
					}
					next := testProviderLayer(t, "first", "new", "New", at.Add(time.Second))
					invalid := testProviderLayer(t, "second", "new", "New", at.Add(time.Second))
					switch defect {
					case "digest":
						invalid.Digest = old.Digest
					case "payload-provider":
						invalid.ProviderID = "different"
					case "invalid-payload":
						invalid.Payload = []byte("not a catalog")
						invalid.Digest = catalogs.DescribeCatalogPayload(invalid.Payload).Checksum
					case "missing-time":
						invalid.ObservedAt = time.Time{}
					case "provider-alias", "multiple-providers":
						invalid = providerIdentityFixture(t, defect, invalid)
					}
					if err := r.retainProviders(t.Context(), []ProviderLayer{next, invalid}); err == nil {
						t.Error("invalid provider evidence accepted")
					}
					if len(r.layers.providers) != 1 || !bytes.Equal(r.layers.providers[old.evidenceKey()].Payload, old.Payload) {
						t.Error("invalid batch changed retained memory")
					}
					if durable {
						retained, err := store.loadProviders()
						if err != nil || len(retained) != 1 || !bytes.Equal(retained[old.evidenceKey()].Payload, old.Payload) {
							t.Error("invalid batch changed durable evidence", err)
						}
					}
				})
			}
		})
	}
}

func providerIdentityFixture(t *testing.T, defect string, layer ProviderLayer) ProviderLayer {
	t.Helper()
	builder := catalogs.NewEmpty()
	provider := catalogs.Provider{ID: layer.ProviderID, Name: "Provider"}
	if defect == "provider-alias" {
		provider.Aliases = []catalogs.ProviderID{"alias"}
		layer.ProviderID = "alias"
	} else if err := builder.SetProvider(catalogs.Provider{ID: "extra", Name: "Extra"}); err != nil {
		t.Fatal(err)
	}
	if err := builder.SetProvider(provider); err != nil {
		t.Fatal(err)
	}
	observed, err := catalogs.NewObservationCatalog(builder)
	if err != nil {
		t.Fatal(err)
	}
	layer.Payload, err = catalogs.EncodeCatalogPayload(observed)
	if err != nil {
		t.Fatal(err)
	}
	layer.Digest = catalogs.DescribeCatalogPayload(layer.Payload).Checksum
	return layer
}

func TestRetainedProviderEvidenceBindsFilenameAndPayload(t *testing.T) {
	for _, defect := range []string{"filename", "digest", "payload-provider", "missing-time"} {
		t.Run(defect, func(t *testing.T) {
			store, err := newLayerStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			layer := testProviderLayer(t, "provider", "model", "Model", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
			name := "provider.json"
			switch defect {
			case "filename":
				name = "different.json"
			case "digest":
				layer.Digest = "sha256:invalid"
			case "payload-provider":
				layer.ProviderID = "different"
				name = "different.json"
			case "missing-time":
				layer.ObservedAt = time.Time{}
			}
			raw, err := json.Marshal(layer)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(store.root, providerLayerDirectoryName, name)
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.loadProviders(); err == nil {
				t.Error("invalid retained provider evidence accepted")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(raw, after) {
				t.Error("refusal changed retained evidence", err)
			}
		})
	}
}

func TestProviderEvidenceRetentionOwnsPayload(t *testing.T) {
	store, err := newLayerStore("")
	if err != nil {
		t.Fatal(err)
	}
	r := &Runtime{store: store}
	layer := testProviderLayer(t, "provider", "model", "Model", time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC))
	want := bytes.Clone(layer.Payload)
	if err := r.retainProviders(t.Context(), []ProviderLayer{layer}); err != nil {
		t.Fatal(err)
	}
	clear(layer.Payload)
	if !bytes.Equal(r.layers.providers[layer.evidenceKey()].Payload, want) {
		t.Fatal("caller mutation changed retained evidence")
	}
}
