package models

import (
	stderrors "errors"
	"testing"

	"github.com/agentstation/starmap/internal/catalog/query"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestExportModelsRejectsMixedProviders(t *testing.T) {
	records := []query.ModelRecord{
		{
			ProviderID: "provider-a",
			Model:      catalogs.Model{ID: "shared", Name: "Provider A"},
		},
		{
			ProviderID: "provider-b",
			Model:      catalogs.Model{ID: "shared", Name: "Provider B"},
		},
	}
	err := exportModels(records, "openrouter")
	var validation *errors.ValidationError
	if !stderrors.As(err, &validation) || validation.Field != "provider" {
		t.Fatalf("exportModels error = %T %v, want provider ValidationError", err, err)
	}
}
