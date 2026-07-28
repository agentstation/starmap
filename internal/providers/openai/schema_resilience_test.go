package openai

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sourcepayload"
)

func TestListModelsQuarantinesMalformedSibling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"id":"valid","owned_by":"openai"},
			{"id":"invalid","max_model_len":"schema-drift"}
		]}`))
	}))
	defer server.Close()

	client, err := NewClient(&catalogs.Provider{
		ID: "openai", Name: "OpenAI",
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{URL: server.URL}},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	models, err := client.ListModels(context.Background())
	var quarantineErr *sourcepayload.QuarantineError
	if !stderrors.As(err, &quarantineErr) {
		t.Fatalf("error = %T: %v, want *sourcepayload.QuarantineError", err, err)
	}
	if len(models) != 1 || models[0].ID != "valid" {
		t.Fatalf("models = %#v, want valid sibling", models)
	}
	if quarantineErr.Report.Rejected != 1 {
		t.Fatalf("report = %#v, want one rejection", quarantineErr.Report)
	}
}
