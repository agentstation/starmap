package anthropic

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentstation/starmap/internal/sourcepayload"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestListModelsQuarantinesMalformedSibling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"id":"valid","display_name":"Valid"},
			{"id":"invalid","display_name":"Invalid","max_tokens":"schema-drift"}
		]}`))
	}))
	defer server.Close()

	client := NewClient(&catalogs.Provider{
		ID: "anthropic", Name: "Anthropic",
		Catalog: &catalogs.ProviderCatalog{Endpoint: catalogs.ProviderEndpoint{URL: server.URL}},
	})
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
