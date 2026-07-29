package catalogs

import "testing"

func TestProvidersCopyOnReadWrite(t *testing.T) {
	docs := "https://example.com/docs"
	provider := &Provider{
		ID:      "provider",
		Aliases: []ProviderID{"provider-alias"},
		Catalog: &ProviderCatalog{Docs: &docs},
		Models: map[string]*Model{
			"model": {
				ID: "model",
				Metadata: &ModelMetadata{
					Tags: []ModelTag{ModelTagCoding},
				},
			},
		},
	}

	providers := NewProviders()
	if err := providers.Set(provider.ID, provider); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	*provider.Catalog.Docs = "mutated input"
	provider.Models["model"].Metadata.Tags[0] = ModelTagMath

	stored, ok := providers.Get("provider")
	if !ok {
		t.Fatal("Expected provider to exist")
	}
	if *stored.Catalog.Docs != "https://example.com/docs" {
		t.Fatal("Set stored caller-owned provider references")
	}
	if stored.Models["model"].Metadata.Tags[0] != ModelTagCoding {
		t.Fatal("Set stored caller-owned model references")
	}

	*stored.Catalog.Docs = "mutated get"
	stored.Models["model"].Metadata.Tags[0] = ModelTagMath

	resolved, ok := providers.Resolve("provider-alias")
	if !ok {
		t.Fatal("Expected alias to resolve")
	}
	if *resolved.Catalog.Docs != "https://example.com/docs" {
		t.Fatal("Get returned provider catalog internals")
	}
	if resolved.Models["model"].Metadata.Tags[0] != ModelTagCoding {
		t.Fatal("Get returned provider model internals")
	}

	mapped := providers.Map()
	*mapped["provider"].Catalog.Docs = "mutated map"

	again, ok := providers.Get("provider")
	if !ok {
		t.Fatal("Expected provider to still exist")
	}
	if *again.Catalog.Docs != "https://example.com/docs" {
		t.Fatal("Map returned provider internals")
	}
}

func TestAuthorsCopyOnReadWrite(t *testing.T) {
	description := "original"
	author := &Author{
		ID:          "author",
		Aliases:     []AuthorID{"author-alias"},
		Description: &description,
		Catalog: &AuthorCatalog{
			Attribution: &AuthorAttribution{
				ProviderID: "provider",
				Patterns:   []string{"model-*"},
			},
		},
	}

	authors := NewAuthors()
	if err := authors.Set(author.ID, author); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	*author.Description = "mutated input"
	author.Catalog.Attribution.Patterns[0] = "mutated-*"

	stored, ok := authors.Get("author")
	if !ok {
		t.Fatal("Expected author to exist")
	}
	if *stored.Description != "original" {
		t.Fatal("Set stored caller-owned author references")
	}
	if stored.Catalog.Attribution.Patterns[0] != "model-*" {
		t.Fatal("Set stored caller-owned attribution references")
	}

	*stored.Description = "mutated get"
	stored.Catalog.Attribution.Patterns[0] = "mutated-*"

	resolved, ok := authors.Resolve("author-alias")
	if !ok {
		t.Fatal("Expected alias to resolve")
	}
	if *resolved.Description != "original" {
		t.Fatal("Get returned author internals")
	}
	if resolved.Catalog.Attribution.Patterns[0] != "model-*" {
		t.Fatal("Get returned author attribution internals")
	}

	mapped := authors.Map()
	*mapped["author"].Description = "mutated map"
	mapped["author"].Catalog.Attribution.Patterns[0] = "mutated-*"

	again, ok := authors.Get("author")
	if !ok {
		t.Fatal("Expected author to still exist")
	}
	if *again.Description != "original" {
		t.Fatal("Map returned author internals")
	}
	if again.Catalog.Attribution.Patterns[0] != "model-*" {
		t.Fatal("Map returned author attribution internals")
	}
}

func TestModelsCopyOnReadWrite(t *testing.T) {
	model := &Model{
		ID: "model",
		Metadata: &ModelMetadata{
			Tags: []ModelTag{ModelTagCoding},
		},
	}

	models := NewModels()
	if err := models.Set(model.ID, model); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	model.Metadata.Tags[0] = ModelTagMath

	stored, ok := models.Get("model")
	if !ok {
		t.Fatal("Expected model to exist")
	}
	if stored.Metadata.Tags[0] != ModelTagCoding {
		t.Fatal("Set stored caller-owned model references")
	}

	stored.Metadata.Tags[0] = ModelTagMath

	again, ok := models.Get("model")
	if !ok {
		t.Fatal("Expected model to still exist")
	}
	if again.Metadata.Tags[0] != ModelTagCoding {
		t.Fatal("Get returned model internals")
	}

	mapped := models.Map()
	mapped["model"].Metadata.Tags[0] = ModelTagMath

	again, ok = models.Get("model")
	if !ok {
		t.Fatal("Expected model to still exist")
	}
	if again.Metadata.Tags[0] != ModelTagCoding {
		t.Fatal("Map returned model internals")
	}
}

func TestEndpointsCopyOnReadWrite(t *testing.T) {
	endpoint := &Endpoint{
		ID:          "endpoint",
		Name:        "Endpoint",
		Description: "original",
	}

	endpoints := NewEndpoints()
	if err := endpoints.Set(endpoint.ID, endpoint); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	endpoint.Description = "mutated input"

	stored, ok := endpoints.Get("endpoint")
	if !ok {
		t.Fatal("Expected endpoint to exist")
	}
	if stored.Description != "original" {
		t.Fatal("Set stored caller-owned endpoint reference")
	}

	stored.Description = "mutated get"

	again, ok := endpoints.Get("endpoint")
	if !ok {
		t.Fatal("Expected endpoint to still exist")
	}
	if again.Description != "original" {
		t.Fatal("Get returned endpoint internals")
	}

	mapped := endpoints.Map()
	mapped["endpoint"].Description = "mutated map"

	again, ok = endpoints.Get("endpoint")
	if !ok {
		t.Fatal("Expected endpoint to still exist")
	}
	if again.Description != "original" {
		t.Fatal("Map returned endpoint internals")
	}

	endpoints.ForEach(func(_ string, endpoint *Endpoint) bool {
		endpoint.Description = "mutated foreach"
		return true
	})

	again, ok = endpoints.Get("endpoint")
	if !ok {
		t.Fatal("Expected endpoint to still exist")
	}
	if again.Description != "original" {
		t.Fatal("ForEach exposed endpoint internals")
	}
}

func TestEndpointsBatchCopyOnWrite(t *testing.T) {
	added := &Endpoint{ID: "added", Name: "Added", Description: "added original"}
	endpoints := NewEndpoints()

	if errs := endpoints.AddBatch([]*Endpoint{added}); len(errs) != 0 {
		t.Fatalf("AddBatch failed: %v", errs)
	}
	added.Description = "mutated added"

	stored, ok := endpoints.Get("added")
	if !ok {
		t.Fatal("Expected added endpoint to exist")
	}
	if stored.Description != "added original" {
		t.Fatal("AddBatch stored caller-owned endpoint reference")
	}

	set := &Endpoint{ID: "set", Name: "Set", Description: "set original"}
	if err := endpoints.SetBatch(map[string]*Endpoint{"set": set}); err != nil {
		t.Fatalf("SetBatch failed: %v", err)
	}
	set.Description = "mutated set"

	stored, ok = endpoints.Get("set")
	if !ok {
		t.Fatal("Expected set endpoint to exist")
	}
	if stored.Description != "set original" {
		t.Fatal("SetBatch stored caller-owned endpoint reference")
	}
}

func TestEndpointsWithMapCopyOnWrite(t *testing.T) {
	endpoint := &Endpoint{ID: "endpoint", Name: "Endpoint", Description: "original"}
	endpoints := NewEndpoints(WithEndpointsMap(map[string]*Endpoint{
		endpoint.ID: endpoint,
	}))

	endpoint.Description = "mutated input"

	stored, ok := endpoints.Get("endpoint")
	if !ok {
		t.Fatal("Expected endpoint to exist")
	}
	if stored.Description != "original" {
		t.Fatal("WithEndpointsMap stored caller-owned endpoint reference")
	}
}
