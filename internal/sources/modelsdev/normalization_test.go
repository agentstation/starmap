package modelsdev

import (
	"context"
	"encoding/json"
	"testing"

	testlogging "github.com/agentstation/starmap/internal/test/logging"
	"github.com/agentstation/starmap/pkg/logging"
	"github.com/agentstation/starmap/pkg/sources"
)

func TestModelsDevDisplayNameNormalizationPreservesIdentityAndReportsCorrections(t *testing.T) {
	for _, transport := range []string{"http", "git"} {
		t.Run(transport, func(t *testing.T) {
			api := API{"novita-ai": Provider{ID: "novita-ai", Name: "Novita", Models: map[string]Model{
				"deepseek/deepseek-r1-turbo": {ID: "deepseek/deepseek-r1-turbo", Name: "DeepSeek R1 (Turbo)\t", Description: "test"},
				"deepseek/deepseek-v3-turbo": {ID: "deepseek/deepseek-v3-turbo", Name: "DeepSeek V3 (Turbo)\t", Description: "test"},
				"sao10K/l3-70b-euryale-v2.1": {ID: "sao10K/l3-70b-euryale-v2.1", Name: "L3 70B Euryale V2.1\t", Description: "test"},
				"sao10K/l3-8b-lunaris":       {ID: "sao10K/l3-8b-lunaris", Name: "Sao10k L3 8B Lunaris\t", Description: "test"},
			}}}
			load := func(context.Context, string) (*API, error) { return &api, nil }
			var source sources.Source = &HTTPSource{loadAPI: load}
			if transport == "git" {
				source = &GitSource{loadAPI: load}
			}
			logger := testlogging.New(t)
			ctx := logging.WithRunID(logging.WithLogger(t.Context(), logger.Logger), "normalization-test")
			observation, err := source.Observe(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if observation.Status != sources.ObservationStatusSucceeded || observation.Records.Accepted != 4 || observation.Records.Rejected != 0 {
				t.Fatalf("normalized records were quarantined: %+v", observation.Records)
			}
			provider, err := observation.Catalog.Provider("novita-ai")
			if err != nil {
				t.Fatal(err)
			}
			for id, model := range api["novita-ai"].Models {
				accepted := provider.Models[id]
				if accepted == nil || accepted.ID != id || accepted.Name != model.Name[:len(model.Name)-1] || model.Name[len(model.Name)-1] != '\t' {
					t.Fatalf("normalization changed identity or original input for %q", id)
				}
			}
			corrections := make(map[string]bool)
			for _, line := range logger.Lines() {
				var event map[string]any
				if err := json.Unmarshal([]byte(line), &event); err != nil {
					t.Fatal(err)
				}
				if event["code"] == "display_name_whitespace_trimmed" {
					id, _ := event["model_id"].(string)
					if provider.Models[id] == nil || event["provider_id"] != "novita-ai" || event["run_id"] != "normalization-test" || event["source"] != source.ID().String() {
						t.Fatalf("correction lost source or record identity: %+v", event)
					}
					corrections[id] = true
				}
			}
			if len(corrections) != 4 {
				t.Fatalf("reported %d corrections, want four", len(corrections))
			}
		})
	}
}
