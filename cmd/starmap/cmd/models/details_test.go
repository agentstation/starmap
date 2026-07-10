package models

import (
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestModelDetailRowsExposeSourceCompleteFields(t *testing.T) {
	root := "gpt-4o"
	parent := "ft:gpt-4o:custom"
	model := &catalogs.Model{
		ID:     "gpt-4o-custom",
		Name:   "GPT-4o Custom",
		Status: catalogs.ModelStatusBeta,
		Lineage: &catalogs.ModelLineage{
			Family: "gpt-4o",
			Root:   &root,
			Parent: &parent,
		},
		Limits: &catalogs.ModelLimits{
			ContextWindow: 128000,
			InputTokens:   96000,
			OutputTokens:  4096,
		},
		Pricing: &catalogs.ModelPricing{
			Tiers: []catalogs.ModelPricingTier{{
				Name: "context_over_200k",
				Type: catalogs.ModelPricingTierTypeContext,
				Size: 200000,
			}},
		},
		Modes: map[string]catalogs.ModelMode{
			"fast":     {},
			"priority": {},
		},
		Extensions: catalogs.SourceExtensions{
			"models.dev": {},
			"openai":     {},
		},
	}
	provider := catalogs.Provider{ID: "openai", Name: "OpenAI"}

	var rows [][]string
	rows = addIdentityRows(rows, model, provider)
	rows = addLimitRows(rows, model)
	rows = addPricingRows(rows, model)
	rows = addModeRows(rows, model)
	rows = addExtensionRows(rows, model)

	assertRowValue(t, rows, "Status", "beta")
	assertRowValue(t, rows, "Family", "gpt-4o")
	assertRowValue(t, rows, "Root Model", root)
	assertRowValue(t, rows, "Parent Model", parent)
	assertRowValue(t, rows, "Max Input", "96,000 tokens")
	assertRowValue(t, rows, "Pricing Tiers", "1 tier")
	assertRowValue(t, rows, "Modes", "fast, priority")
	assertRowValue(t, rows, "Source Extensions", "models.dev, openai")
}

func assertRowValue(t *testing.T, rows [][]string, property, value string) {
	t.Helper()
	for _, row := range rows {
		if len(row) == 2 && row[0] == property {
			if row[1] != value {
				t.Fatalf("%s = %q, want %q", property, row[1], value)
			}
			return
		}
	}
	t.Fatalf("missing row %q in %v", property, rows)
}
