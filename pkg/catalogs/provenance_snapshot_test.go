package catalogs

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/provenance"
)

func TestImmutableProvenanceOwnsNestedValues(t *testing.T) {
	for _, observation := range []bool{false, true} {
		name := "catalog"
		if observation {
			name = "observation"
		}
		t.Run(name, func(t *testing.T) {
			input := map[string]any{"nested": []any{map[string]any{"name": "original"}}}
			previous := map[string]any{"name": "previous"}
			rejections := []provenance.Rejection{{Reason: "original rejection"}}
			builder := NewEmpty()
			if err := builder.SetAuthor(Author{ID: "author", Name: "Author"}); err != nil {
				t.Fatal(err)
			}
			const key = "author:author:metadata"
			builder.SetProvenance(provenance.Map{key: {{Field: "metadata", Value: input, PreviousValue: previous, Rejections: rejections}}})
			var catalog *Catalog
			var err error
			if observation {
				catalog, err = NewObservationCatalog(builder)
			} else {
				catalog, err = builder.Build()
			}
			if err != nil {
				t.Fatal(err)
			}
			want, err := EncodeCatalogPayload(catalog)
			if err != nil {
				t.Fatal(err)
			}
			input["nested"].([]any)[0].(map[string]any)["name"] = "changed input"
			previous["name"] = "changed previous"
			rejections[0].Reason = "changed rejection"
			reader := catalog.Provenance()
			for _, entries := range [][]provenance.Entry{
				reader.Map()[key],
				reader.FindByField(evidence.ResourceTypeAuthor, "author", "metadata"),
				reader.FindByResource(evidence.ResourceTypeAuthor, "author")["metadata"],
			} {
				if len(entries) != 1 {
					t.Fatal("provenance entry disappeared")
				}
				entry := entries[0]
				value := entry.Value.(map[string]any)["nested"].([]any)[0].(map[string]any)
				if value["name"] != "original" || entry.PreviousValue.(map[string]any)["name"] != "previous" || entry.Rejections[0].Reason != "original rejection" {
					t.Fatal("immutable provenance retained mutable input references")
				}
				value["name"] = "changed read"
				entry.PreviousValue.(map[string]any)["name"] = "changed read"
				entry.Rejections[0].Reason = "changed read"
			}
			fresh, err := encodeCatalogPayload(catalog)
			if err != nil || !bytes.Equal(fresh, want) {
				t.Fatalf("catalog reads diverged from its encoded payload: %v", err)
			}
		})
	}
}

type mutableEvidence struct{ values map[string]any }

func (m *mutableEvidence) MarshalJSON() ([]byte, error) { return json.Marshal(m.values) }

func TestImmutableProvenanceSnapshotsCustomValues(t *testing.T) {
	value := &mutableEvidence{values: map[string]any{"count": json.Number("18446744073709551615")}}
	builder := NewEmpty()
	builder.SetProvenance(provenance.Map{"author:author:custom": {{Value: value}}})
	want, err := EncodeCatalogPayload(builder)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatal(err)
	}
	value.values["count"] = 0
	var workers sync.WaitGroup
	for range 16 {
		workers.Go(func() {
			data := catalog.Provenance().Map()
			data["author:author:custom"][0].Value.(map[string]any)["count"] = "caller value"
			encoded, err := EncodeCatalogPayload(catalog)
			if err != nil || !bytes.Equal(encoded, want) {
				t.Errorf("custom evidence changed after publication: %v", err)
			}
		})
	}
	workers.Wait()
}

func TestImmutableProvenanceRejectsCycles(t *testing.T) {
	mapping := map[string]any{}
	mapping["self"] = mapping
	sequence := make([]any, 1)
	sequence[0] = sequence
	for _, value := range []any{mapping, sequence, &mutableEvidence{values: mapping}} {
		builder := NewEmpty()
		builder.SetProvenance(provenance.Map{"author:author:cycle": {{Value: value}}})
		if _, err := builder.Build(); err == nil {
			t.Fatal("immutable catalog accepted cyclic provenance")
		}
		if _, err := NewObservationCatalog(builder); err == nil {
			t.Fatal("immutable observation accepted cyclic provenance")
		}
	}
}
