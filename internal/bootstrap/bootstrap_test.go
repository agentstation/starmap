package bootstrap

import (
	"bytes"
	stderrors "errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/internal/catalog/workspace"
	"github.com/agentstation/starmap/internal/embedded"
	"github.com/agentstation/starmap/pkg/catalogs"
	sourcepayload "github.com/agentstation/starmap/pkg/sources/payload"
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
	manifest, err := Load(catalog)
	if err != nil {
		t.Fatalf("Load: %v; actual descriptor: %#v", err, catalogs.DescribeCatalogPayload(payload))
	}
	semanticChecksum, err := catalogs.CatalogSemanticChecksum(catalog)
	if err != nil {
		t.Fatalf("CatalogSemanticChecksum: %v", err)
	}
	if manifest.SemanticChecksum != semanticChecksum ||
		manifest.Payload != catalogs.DescribeCatalogPayload(payload) {
		t.Fatalf(
			"manifest identities = (%q, %#v), want (%q, %#v)",
			manifest.SemanticChecksum,
			manifest.Payload,
			semanticChecksum,
			catalogs.DescribeCatalogPayload(payload),
		)
	}
	generation, err := Generation()
	if err != nil {
		t.Fatalf("Generation: %v", err)
	}
	if generation.Manifest.GenerationID != manifest.GenerationID ||
		generation.Manifest.GeneratedAt != manifest.GeneratedAt ||
		generation.Manifest.SchemaVersion != manifest.SchemaVersion ||
		generation.Manifest.Payload != manifest.Payload ||
		!bytes.Equal(generation.Payload, payload) {
		t.Fatalf("generation = %#v, bootstrap manifest = %#v", generation.Manifest, manifest)
	}
}

func TestEmbeddedReturnsOneVerifiedImmutableCatalog(t *testing.T) {
	first, firstManifest, err := Embedded()
	if err != nil {
		t.Fatalf("Embedded first call: %v", err)
	}
	second, secondManifest, err := Embedded()
	if err != nil {
		t.Fatalf("Embedded second call: %v", err)
	}
	if first == nil || first != second {
		t.Fatalf("Embedded catalogs = %p and %p, want one non-nil immutable instance", first, second)
	}
	if firstManifest != secondManifest {
		t.Fatalf("Embedded manifests differ: %#v != %#v", firstManifest, secondManifest)
	}
	if firstManifest.GenerationID == "" || firstManifest.Payload.Checksum == "" {
		t.Fatalf("Embedded manifest is not verified: %#v", firstManifest)
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

func TestEmbeddedGenerationPayloadRoundTripsWithoutQuarantine(t *testing.T) {
	generation, err := Generation()
	if err != nil {
		t.Fatalf("Generation: %v", err)
	}
	catalog, err := catalogs.DecodeCatalogPayload(generation.Payload)
	if err != nil {
		var quarantine *sourcepayload.QuarantineError
		if stderrors.As(err, &quarantine) {
			t.Fatalf(
				"DecodeCatalogPayload quarantined %d records; first issue %s: %v",
				quarantine.Report.Rejected,
				quarantine.Report.Issues[0].Subject,
				quarantine.Report.Issues[0].Err,
			)
		}
		t.Fatalf("DecodeCatalogPayload: %v", err)
	}
	if catalog == nil {
		t.Fatal("decoded catalog is nil")
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
		{
			name:       "Groq instant label joins Meta Llama Instruct",
			providerID: "groq", providerModelID: "llama-3.1-8b-instant",
			definitionID: "meta/llama-3.1-8b-instruct",
		},
		{
			name:       "Groq versatile label joins Meta Llama Instruct",
			providerID: "groq", providerModelID: "llama-3.3-70b-versatile",
			definitionID: "meta/llama-3.3-70b-instruct",
		},
		{
			name:       "DeepInfra GPT Turbo tier joins base model",
			providerID: "deepinfra", providerModelID: "openai/gpt-oss-120b-Turbo",
			definitionID: "openai/gpt-oss-120b",
		},
		{
			name:       "DeepInfra Qwen Turbo tier joins base model",
			providerID: "deepinfra", providerModelID: "Qwen/Qwen3-Coder-480B-A35B-Instruct-Turbo",
			definitionID: "qwen/qwen3-coder-480b-a35b-instruct",
		},
		{
			name:       "DeepInfra MiniMax Turbo tier joins base model",
			providerID: "deepinfra", providerModelID: "MiniMaxAI/MiniMax-M2.7-Turbo",
			definitionID: "minimax/minimax-m2.7",
		},
		{
			name:       "DeepInfra Gemma Turbo tier joins base model",
			providerID: "deepinfra", providerModelID: "google/gemma-4-31B-it-turbo",
			definitionID: "google/gemma-4-31b-it",
		},
		{
			name:       "DeepInfra FP8 route joins base Meta model",
			providerID: "deepinfra", providerModelID: "meta-llama/Llama-4-Maverick-17B-128E-Instruct-FP8",
			definitionID: "meta/llama-4-maverick-17b-128e-instruct",
		},
		{
			name:       "Fireworks FP8 route joins base FLUX model",
			providerID: "fireworks-ai", providerModelID: "accounts/fireworks/models/flux-1-schnell-fp8",
			definitionID: "black-forest-labs/flux-1-schnell",
		},
		{
			name:       "Vertex embedding alias joins EmbeddingGemma 300M",
			providerID: "google-vertex", providerModelID: "embeddinggemma",
			definitionID: "google/embeddinggemma-300m",
		},
		{
			name:       "Nano Banana joins Gemini image model",
			providerID: "google-ai-studio", providerModelID: "nano-banana-pro-preview",
			definitionID: "google/gemini-3-pro-image-preview",
		},
		{
			name:       "DeepInfra GTE route joins Alibaba model",
			providerID: "deepinfra", providerModelID: "thenlper/gte-base",
			definitionID: "alibaba/gte-base",
		},
		{
			name:       "FastVideo route joins Lightricks model",
			providerID: "deepinfra", providerModelID: "FastVideo/LTX-2.3-Distilled-Diffusers",
			definitionID: "lightricks/ltx-2.3-distilled-diffusers",
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

func TestEmbeddedOfferingsDoNotPublishChatRoutesForNonChatOperations(t *testing.T) {
	builder, err := catalogs.NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	for _, test := range []struct {
		providerID catalogs.ProviderID
		modelID    catalogs.ProviderModelID
	}{
		{providerID: "deepinfra", modelID: "BAAI/bge-m3-multi"},
		{providerID: "deepinfra", modelID: "FastVideo/LTX-2.3-Distilled-Diffusers"},
		{providerID: "deepinfra", modelID: "black-forest-labs/FLUX-1-dev"},
	} {
		offering, err := catalog.Offering(test.providerID, test.modelID)
		if err != nil {
			t.Fatalf("Offering(%s, %s): %v", test.providerID, test.modelID, err)
		}
		if len(offering.Endpoints) != 0 {
			t.Fatalf(
				"Offering(%s, %s) endpoints = %#v, want no chat route",
				test.providerID,
				test.modelID,
				offering.Endpoints,
			)
		}
	}

	chat, err := catalog.Offering("deepinfra", "meta-llama/Llama-3.3-70B-Instruct-Turbo")
	if err != nil {
		t.Fatalf("chat Offering: %v", err)
	}
	chatEndpoint, found := chat.Endpoint(catalogs.ProviderOperationChatCompletions)
	if !found || chatEndpoint.URL != "https://api.deepinfra.com/v1/openai/chat/completions" {
		t.Fatalf("chat endpoint = %#v, found = %t, want DeepInfra chat completions URL", chatEndpoint, found)
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
