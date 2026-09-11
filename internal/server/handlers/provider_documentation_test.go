package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/server/cache"
	testcatalog "github.com/agentstation/starmap/internal/test/catalog"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestProviderListPreservesDocumentationAcrossCache(t *testing.T) {
	baseline, err := testcatalog.EmbeddedBuilder()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, general, acquisition, want string
	}{
		{name: "general without acquisition", general: "https://example.test/docs", want: "https://example.test/docs"},
		{name: "general takes precedence", general: "https://example.test/docs", acquisition: "https://example.test/models", want: "https://example.test/docs"},
		{name: "acquisition fallback", acquisition: "https://example.test/models", want: "https://example.test/models"},
		{name: "absent"},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider, err := baseline.Provider("openai")
			if err != nil {
				t.Fatal(err)
			}
			provider.Models = nil
			provider.DocsURL = nil
			if test.general != "" {
				docs := test.general
				provider.DocsURL = &docs
			}
			if test.acquisition == "" {
				provider.Catalog = nil
			} else {
				docs := test.acquisition
				provider.Catalog.Docs = &docs
			}
			if err := provider.ValidateContract(); err != nil {
				t.Fatal(err)
			}
			builder := catalogs.NewEmpty()
			for _, author := range baseline.Authors().List() {
				if err := builder.SetAuthor(author); err != nil {
					t.Fatal(err)
				}
			}
			if err := builder.SetProvider(provider); err != nil {
				t.Fatal(err)
			}
			catalog, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			h := &Handlers{
				app: &testApplication{CatalogStateFunc: func() (starmap.CatalogState, error) {
					return starmap.CatalogState{Catalog: catalog, Sequence: 1, GenerationID: "provider-docs"}, nil
				}},
				cache: cache.New(time.Minute, 0),
			}
			for _, responseKind := range []string{"fresh", "cached"} {
				recorder := httptest.NewRecorder()
				h.HandleListProviders(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil))
				if recorder.Code != http.StatusOK {
					t.Fatalf("%s status = %d: %s", responseKind, recorder.Code, recorder.Body)
				}
				var result struct {
					Data struct {
						Providers []map[string]any `json:"providers"`
					} `json:"data"`
				}
				if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if len(result.Data.Providers) != 1 {
					t.Fatalf("%s providers = %v", responseKind, result.Data.Providers)
				}
				docs, present := result.Data.Providers[0]["docs_url"]
				if test.want == "" && present || test.want != "" && docs != test.want {
					t.Errorf("%s docs_url = %v (present %t), want %q", responseKind, docs, present, test.want)
				}
				if _, found := h.cache.GetGeneration(1, "provider-docs", "providers"); !found {
					t.Fatal("provider response did not enter its generation cache")
				}
			}
		})
	}
}
