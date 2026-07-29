package bootstrap

import (
	"bytes"
	"io/fs"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/internal/embedded"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestEmbeddedBootstrapManifestMatchesCanonicalCatalog(t *testing.T) {
	builder, err := catalogs.NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if definitions := catalog.Definitions(); len(definitions) == 0 {
		t.Fatal("embedded catalog published no canonical definitions")
	}
	offeringCount := 0
	for _, provider := range catalog.Providers().List() {
		offerings, err := catalog.ProviderOfferings(provider.ID)
		if err != nil {
			t.Fatalf("ProviderOfferings(%s): %v", provider.ID, err)
		}
		for _, offering := range offerings {
			if err := offering.Validate(); err != nil {
				t.Fatalf("Offering(%s/%s): %v", offering.ProviderID, offering.ProviderModelID, err)
			}
		}
		offeringCount += len(offerings)
	}
	if offeringCount == 0 {
		t.Fatal("embedded catalog published no provider offerings")
	}
	groqGPTOSS, err := catalog.Offering("groq", "openai/gpt-oss-120b")
	if err != nil {
		t.Fatalf("Groq GPT OSS offering: %v", err)
	}
	if groqGPTOSS.Pricing == nil || groqGPTOSS.Pricing.Tokens == nil ||
		groqGPTOSS.Pricing.Tokens.Input == nil ||
		groqGPTOSS.Pricing.Tokens.Output == nil ||
		groqGPTOSS.Pricing.Tokens.CacheRead == nil ||
		groqGPTOSS.Pricing.Tokens.Input.Per1M != 0.15 ||
		groqGPTOSS.Pricing.Tokens.Output.Per1M != 0.60 ||
		groqGPTOSS.Pricing.Tokens.CacheRead.Per1M != 0.075 {
		t.Fatalf("Groq GPT OSS per-million pricing = %#v", groqGPTOSS.Pricing)
	}
	providerYAMLCount := 0
	if err := fs.WalkDir(embedded.FS, "catalog/providers", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.Contains(path, "/models/") && strings.HasSuffix(path, ".yaml") {
			providerYAMLCount++
		}
		return nil
	}); err != nil {
		t.Fatalf("walk embedded provider YAML: %v", err)
	}
	if offeringCount != providerYAMLCount {
		t.Fatalf("published offerings = %d, embedded provider-model YAML files = %d", offeringCount, providerYAMLCount)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	if _, err := Load(catalog); err != nil {
		t.Fatalf("Load: %v; actual descriptor: %#v", err, catalogs.DescribeCatalogPayload(payload))
	}
}

func TestEmbeddedBootstrapArtifactGenerationIsDeterministic(t *testing.T) {
	first, err := Generation()
	if err != nil {
		t.Fatalf("Generation first: %v", err)
	}
	second, err := Generation()
	if err != nil {
		t.Fatalf("Generation second: %v", err)
	}
	if first.Manifest.GenerationID != second.Manifest.GenerationID ||
		first.Manifest.Payload != second.Manifest.Payload || string(first.Payload) != string(second.Payload) {
		t.Fatalf("embedded generations differ: %#v / %#v", first.Manifest, second.Manifest)
	}
}

func TestEmbeddedCatalogIdentityGraphAndEndpointProjectionAreComplete(t *testing.T) {
	builder, err := catalogs.NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	definitions := catalog.Definitions()
	definitionIDs := make(map[catalogs.ModelDefinitionID]struct{}, len(definitions))
	for _, definition := range definitions {
		definitionIDs[definition.ID] = struct{}{}
	}
	for _, definition := range definitions {
		for field, related := range map[string]*catalogs.ModelDefinitionID{
			"root": definition.Lineage.Root, "parent": definition.Lineage.Parent,
		} {
			if related == nil {
				continue
			}
			if _, found := definitionIDs[*related]; !found {
				t.Fatalf("%s lineage %s references missing definition %s", definition.ID, field, *related)
			}
		}
	}

	authorYAMLCount := countEmbeddedModelYAML(t, "catalog/authors")
	if len(definitions) != authorYAMLCount {
		t.Fatalf("definitions = %d, embedded author-model YAML files = %d", len(definitions), authorYAMLCount)
	}

	generation, err := Generation()
	if err != nil {
		t.Fatalf("Generation: %v", err)
	}
	generated, err := workspace.EncodeEndpointProjection(catalog, workspace.Identity{
		GenerationID:    generation.Manifest.GenerationID,
		PayloadChecksum: generation.Manifest.Payload.Checksum,
	})
	if err != nil {
		t.Fatalf("EncodeEndpointProjection: %v", err)
	}
	embeddedProjection, err := fs.ReadFile(embedded.FS, "catalog/endpoints.yaml")
	if err != nil {
		t.Fatalf("read embedded endpoint projection: %v", err)
	}
	if !bytes.Equal(generated, embeddedProjection) {
		t.Fatal("embedded endpoints.yaml is not the exact deterministic projection of the embedded catalog")
	}

	var projection struct {
		Models []struct {
			Model     catalogs.ModelDefinitionID `yaml:"model"`
			Endpoints []struct {
				ProviderID      catalogs.ProviderID      `yaml:"provider"`
				ProviderModelID catalogs.ProviderModelID `yaml:"provider_model_id"`
			} `yaml:"endpoints"`
		} `yaml:"models"`
	}
	if err := yaml.Unmarshal(embeddedProjection, &projection); err != nil {
		t.Fatalf("decode embedded endpoint projection: %v", err)
	}
	endpointRows := 0
	for _, model := range projection.Models {
		if _, found := definitionIDs[model.Model]; !found {
			t.Fatalf("endpoint projection references missing definition %s", model.Model)
		}
		endpointRows += len(model.Endpoints)
	}
	providerYAMLCount := countEmbeddedModelYAML(t, "catalog/providers")
	if endpointRows != providerYAMLCount {
		t.Fatalf("endpoint rows = %d, embedded provider-model YAML files = %d", endpointRows, providerYAMLCount)
	}
}

func TestEmbeddedProviderIdentityIsIndependentFromModelAuthorship(t *testing.T) {
	builder, err := catalogs.NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	for _, test := range []struct {
		name            string
		providerID      catalogs.ProviderID
		providerModelID catalogs.ProviderModelID
		definitionID    catalogs.ModelDefinitionID
	}{
		{
			name:       "Groq serves an OpenAI safety model",
			providerID: "groq", providerModelID: "openai/gpt-oss-safeguard-20b",
			definitionID: "openai/gpt-oss-safeguard-20b",
		},
		{
			name:       "Groq serves a Canopy Labs speech model",
			providerID: "groq", providerModelID: "canopylabs/orpheus-v1-english",
			definitionID: "canopy-labs/orpheus-v1-english",
		},
		{
			name:       "Vertex serves a Meta model",
			providerID: "google-vertex", providerModelID: "bart-large-cnn",
			definitionID: "meta/bart-large-cnn",
		},
		{
			name:       "Alibaba Cloud serves a Qwen Team model",
			providerID: "alibaba", providerModelID: "qwen3-32b",
			definitionID: "qwen/qwen3-32b",
		},
		{
			name:       "DeepInfra serves the same Qwen Team model",
			providerID: "deepinfra", providerModelID: "Qwen/Qwen3-32B",
			definitionID: "qwen/qwen3-32b",
		},
		{
			name:       "Vertex deployment revision joins the OpenAI model",
			providerID: "google-vertex", providerModelID: "gpt-4o-2024-08-06@001",
			definitionID: "openai/gpt-4o-2024-08-06",
		},
		{
			name:       "Vertex MaaS slug joins the Google model",
			providerID: "google-vertex", providerModelID: "gemma-4-26b-a4b-it-maas",
			definitionID: "google/gemma-4-26b-a4b-it",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			offering, err := catalog.Offering(test.providerID, test.providerModelID)
			if err != nil {
				t.Fatalf("Offering(%s, %s): %v", test.providerID, test.providerModelID, err)
			}
			if offering.ProviderModelID != test.providerModelID {
				t.Fatalf("provider model ID = %q, want exact opaque ID %q", offering.ProviderModelID, test.providerModelID)
			}
			if offering.DefinitionID != test.definitionID {
				t.Fatalf("definition ID = %q, want %q", offering.DefinitionID, test.definitionID)
			}
		})
	}

	for definitionID, wantOfferings := range map[catalogs.ModelDefinitionID]int{
		"openai/gpt-4o-2024-08-06":  2,
		"google/gemma-4-26b-a4b-it": 3,
		"qwen/qwen3-32b":            3,
	} {
		offerings, err := catalog.DefinitionOfferings(definitionID)
		if err != nil {
			t.Fatalf("DefinitionOfferings(%s): %v", definitionID, err)
		}
		if len(offerings) != wantOfferings {
			t.Fatalf("DefinitionOfferings(%s) = %d, want %d", definitionID, len(offerings), wantOfferings)
		}
	}
}

func countEmbeddedModelYAML(t testing.TB, root string) int {
	t.Helper()
	count := 0
	if err := fs.WalkDir(embedded.FS, root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.Contains(path, "/models/") && strings.HasSuffix(path, ".yaml") {
			count++
		}
		return nil
	}); err != nil {
		t.Fatalf("walk embedded model YAML under %s: %v", root, err)
	}
	return count
}
