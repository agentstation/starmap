package reconciler

import (
	"encoding/json"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/authority"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestArchitecturePresenceReconciliation(t *testing.T) {
	for _, field := range []string{"quantized", "fine_tuned"} {
		for _, claim := range []string{"missing", "null", "false", "true"} {
			t.Run(field+"/"+claim, func(t *testing.T) {
				body := "{}"
				if claim != "missing" {
					body = `{"` + field + `":` + claim + `}`
				}
				architecture := &catalogs.ModelArchitecture{}
				if err := json.Unmarshal([]byte(body), architecture); err != nil {
					t.Fatal(err)
				}
				policies := authority.New()
				merger := newMerger(policies, NewAuthorityStrategy(policies), nil)
				models, _, err := merger.Models(map[sources.ID][]*catalogs.Model{sources.ModelsDevHTTPID: {{ID: "model", Name: "Model", Metadata: &catalogs.ModelMetadata{Architecture: architecture}}}})
				if err != nil {
					t.Fatal(err)
				}
				if len(models) != 1 || models[0].Metadata == nil {
					t.Fatal("model metadata is missing")
				}
				data, err := json.Marshal(models[0].Metadata.Architecture)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(data, &fields); err != nil {
					t.Fatal(err)
				}
				actual, present := fields[field]
				if claim == "missing" {
					if present {
						t.Errorf("missing claim became %s", actual)
					}
				} else if !present || string(actual) != claim {
					t.Errorf("claim=%s/%t, want %s", actual, present, claim)
				}
			})
		}
	}
}
