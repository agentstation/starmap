package reconciler

import (
	"encoding/json"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/sources"
	"testing"
)

func TestArchitecturePresenceSourcePrecedence(t *testing.T) {
	for _, field := range []string{"quantized", "fine_tuned"} {
		for _, upstream := range []string{"missing", "null", "false", "true"} {
			for _, local := range []string{"missing", "null", "false", "true"} {
				t.Run(field+"/"+upstream+"/"+local, func(t *testing.T) {
					model := func(claim string) *catalogs.Model {
						body := "{}"
						if claim != "missing" {
							body = `{"` + field + `":` + claim + `}`
						}
						a := &catalogs.ModelArchitecture{}
						if err := json.Unmarshal([]byte(body), a); err != nil {
							t.Fatal(err)
						}
						return &catalogs.Model{ID: "model", Name: "Model", Metadata: &catalogs.ModelMetadata{Architecture: a}}
					}
					policies := authority.New()
					m := newMerger(policies, NewAuthorityStrategy(policies), nil)
					models, _, err := m.Models(map[sources.ID][]*catalogs.Model{sources.ModelsDevHTTPID: {model(upstream)}, sources.LocalCatalogID: {model(local)}})
					if err != nil {
						t.Fatal(err)
					}
					if len(models) != 1 {
						t.Fatalf("models=%d", len(models))
					}
					value, presence := architectureProbeClaim(t, models[0], field)
					want := "missing"
					switch {
					case upstream == "true" || upstream == "false":
						want = upstream
					case local == "true" || local == "false":
						want = local
					case upstream == "null" || local == "null":
						want = "null"
					}
					state := catalogs.ValueMissing
					if want == "null" {
						state = catalogs.ValueUnknown
					} else if want != "missing" {
						state = catalogs.ValueKnown
					}
					if value != (want == "true") || presence != state {
						t.Fatalf("got %v/%v, want %s/%v", value, presence, want, state)
					}
				})
			}
		}
	}
}
