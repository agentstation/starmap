package providers_test

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/agentstation/starmap/internal/providers/registry"
	"github.com/agentstation/starmap/internal/sources/nativeproviders"
	"github.com/agentstation/starmap/pkg/catalogs"
)

func TestProviderModuleAndFixtureContracts(t *testing.T) {
	builder, err := catalogs.NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	classCounts := make(map[fixtureClass]int)
	for _, provider := range builder.Providers().List() {
		t.Run(provider.ID.String(), func(t *testing.T) {
			if provider.Catalog == nil || len(provider.Catalog.Sources) == 0 {
				t.Fatal("provider has no configured source")
			}
			for _, source := range provider.Catalog.Sources {
				if !registry.Supports(source.Endpoint.Type) && !nativeproviders.Supports(source.Endpoint.Type) && source.Endpoint.Type != catalogs.EndpointTypeApplication {
					t.Fatalf("source %q endpoint type %q has no production acquisition owner", source.ID, source.Endpoint.Type)
				}
				class := deriveFixtureClass(t, provider, source)
				classCounts[class]++
				switch class {
				case fixtureProhibitedCredentialCapture:
					assertNoGovernedCredentialScopedCapture(t, provider.ID.String(), source.ID)
				case fixtureGovernedRawObservation:
					assertGovernedObservation(t, provider.ID.String(), source.ID)
				case fixtureTableComposition:
					assertTableDrivenConfigurationOnly(t, provider.ID.String())
				case fixtureProviderDelta:
					assertDeterministicProviderFixture(t, provider.ID.String())
				case fixtureSharedConnector:
					assertSharedConnectorFixture(t, source.Endpoint.Type)
				case fixtureSDKFake:
					assertSDKFake(t)
					if source.ObservationScope.Invariant == catalogs.ProviderObservationScopeCredentialScoped ||
						source.ObservationScope.Authenticated == catalogs.ProviderObservationScopeCredentialScoped {
						assertNoGovernedCredentialScopedCapture(t, provider.ID.String(), source.ID)
					}
				default:
					t.Fatalf("source %q has unknown derived fixture class %q", source.ID, class)
				}
			}
		})
	}
	for _, class := range allFixtureClasses {
		if classCounts[class] == 0 {
			t.Fatalf("fixture derivation did not exercise class %q", class)
		}
	}
	assertProviderPackageStructure(t)
	assertConnectorCoverage(t, builder)
	assertProviderRegistry(t)
}

type fixtureClass string

const (
	fixtureSharedConnector             fixtureClass = "shared_connector_fixture"
	fixtureTableComposition            fixtureClass = "table_driven_provider_composition"
	fixtureProviderDelta               fixtureClass = "provider_delta_fixture"
	fixtureGovernedRawObservation      fixtureClass = "governed_raw_public_observation"
	fixtureSDKFake                     fixtureClass = "sdk_fake"
	fixtureProhibitedCredentialCapture fixtureClass = "prohibited_credential_scoped_capture"
)

var allFixtureClasses = []fixtureClass{
	fixtureSharedConnector,
	fixtureTableComposition,
	fixtureProviderDelta,
	fixtureGovernedRawObservation,
	fixtureSDKFake,
	fixtureProhibitedCredentialCapture,
}

func deriveFixtureClass(t *testing.T, provider catalogs.Provider, source catalogs.ProviderSource) fixtureClass {
	t.Helper()
	if nativeproviders.Supports(source.Endpoint.Type) {
		return fixtureSDKFake
	}
	policy := source.ObservationScope
	if policy.Invariant == catalogs.ProviderObservationScopeCredentialScoped ||
		policy.Authenticated == catalogs.ProviderObservationScopeCredentialScoped {
		return fixtureProhibitedCredentialCapture
	}
	if source.Endpoint.Type == catalogs.EndpointTypeApplication {
		return fixtureTableComposition
	}
	if (provider.ID == catalogs.ProviderIDOpenAI || provider.ID == catalogs.ProviderIDAnthropic) && source.ID == "models" {
		return fixtureGovernedRawObservation
	}
	if hasProviderProductionGo(t, provider.ID.String()) || sourceHasWireDelta(source) {
		return fixtureProviderDelta
	}
	if source.Endpoint.Type == catalogs.EndpointTypeOpenAI {
		return fixtureTableComposition
	}
	if source.Endpoint.Type == catalogs.EndpointTypeAnthropic ||
		source.Endpoint.Type == catalogs.EndpointTypeGoogle ||
		source.Endpoint.Type == catalogs.EndpointTypeGoogleCloud {
		return fixtureSharedConnector
	}
	return fixtureProviderDelta
}

func sourceHasWireDelta(source catalogs.ProviderSource) bool {
	endpoint := source.Endpoint
	return endpoint.ResponseCollection != "" || len(endpoint.FieldMappings) != 0 || len(endpoint.FeatureRules) != 0
}

func hasProviderProductionGo(t *testing.T, providerID string) bool {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(providerPackageDirectory(providerID), "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file, "_test.go") {
			return true
		}
	}
	return false
}

func assertDeterministicProviderFixture(t *testing.T, providerID string) {
	t.Helper()
	providerDirectory := providerPackageDirectory(providerID)
	production, err := filepath.Glob(filepath.Join(providerDirectory, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	productionCount := 0
	for _, file := range production {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		productionCount++
		testFile := strings.TrimSuffix(file, ".go") + "_test.go"
		if _, err := os.Stat(testFile); err != nil {
			t.Fatalf("provider delta production file %s requires local behavior test %s: %v", file, testFile, err)
		}
	}
	if productionCount == 0 {
		if _, err := os.Stat(filepath.Join(providerDirectory, "provider_test.go")); err != nil {
			t.Fatalf("declarative provider delta requires provider_test.go: %v", err)
		}
		if _, err := os.Stat(filepath.Join(providerDirectory, "testdata", "models_list.json")); err != nil {
			t.Fatalf("declarative provider delta requires deterministic fixture: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(providerDirectory, "testdata", "models_list.metadata.json")); !os.IsNotExist(err) {
		t.Fatalf("deterministic fixture must not claim observation metadata: %v", err)
	}
}

func assertTableDrivenConfigurationOnly(t *testing.T, providerID string) {
	t.Helper()
	for _, path := range []string{
		filepath.Join(providerID, "provider_test.go"),
		filepath.Join(providerID, "testdata", "models_list.json"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("exact OpenAI provider must use the table-driven configuration contract, not %s: %v", path, err)
		}
	}
}

func assertNoGovernedCredentialScopedCapture(t *testing.T, providerID, sourceID string) {
	t.Helper()
	path := filepath.Join("fixtures", "responses", providerID, sourceID, "models_list.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("credential-scoped raw capture is prohibited: %s: %v", path, err)
	}
}

func assertGovernedObservation(t *testing.T, providerID, sourceID string) {
	t.Helper()
	for _, name := range []string{"models_list.json", "models_list.metadata.json"} {
		if _, err := os.Stat(filepath.Join("fixtures", "responses", providerID, sourceID, name)); err != nil {
			t.Fatalf("governed observation %s: %v", name, err)
		}
	}
}

func assertSharedConnectorFixture(t *testing.T, endpointType catalogs.EndpointType) {
	t.Helper()
	connector := string(endpointType)
	if endpointType == catalogs.EndpointTypeGoogleCloud {
		connector = "google"
	}
	for _, name := range []string{"client_test.go", "response_schema_test.go"} {
		if _, err := os.Stat(filepath.Join("..", "connectors", connector, name)); err != nil {
			t.Fatalf("shared connector %q requires %s: %v", connector, name, err)
		}
	}
}

func providerPackageDirectory(providerID string) string {
	if providerID == catalogs.ProviderIDCloudflare.String() {
		return "cloudflare"
	}
	return providerID
}

func assertSDKFake(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(filepath.Join("..", "sources", "nativeproviders", "source_test.go")); err != nil {
		t.Fatalf("native source requires SDK-fake coverage: %v", err)
	}
}

func TestProviderDocumentationCoversEveryRoleAndRefreshEntryPoint(t *testing.T) {
	for _, document := range []string{filepath.Join("..", "..", "docs", "ADDING_PROVIDERS.md"), filepath.Join("..", "..", "AGENTS.md")} {
		data, err := os.ReadFile(document)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", document, err)
		}
		text := string(data)
		for _, required := range []string{"YAML-only", "connector", "adapter", "client.go", "provider_test.go", "source.go", "pricing.go", "response_schema_test.go", "make testdata PROVIDER=", "provider-contract-check"} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s does not document %q", document, required)
			}
		}
		if strings.Contains(text, "go test ./internal/providers/<provider> -update") {
			t.Fatalf("%s promises the removed test-only refresh path", document)
		}
		for _, forbidden := range []string{"internal/providers/fixtures/policy.yaml", "CustomerInventory", "PublicCatalog"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s revives removed provider contract %q", document, forbidden)
			}
		}
	}
}

func TestProviderConfigurationOwnsStaticOfferingDefaults(t *testing.T) {
	builder, err := catalogs.NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	tests := []struct {
		providerID  catalogs.ProviderID
		baseURL     string
		path        string
		deployment  catalogs.ProviderDeployment
		apis        []catalogs.InvocationAPI
		routability catalogs.OfferingRoutability
	}{
		{catalogs.ProviderIDAnthropic, "https://api.anthropic.com/v1", "", catalogs.ProviderDeployment{Type: "serverless"}, []catalogs.InvocationAPI{catalogs.InvocationAPIMessages}, catalogs.OfferingRoutabilityRoutable},
		{catalogs.ProviderIDCloudflare, "https://api.cloudflare.com/client/v4", "/accounts/{account_id}/ai/v1", catalogs.ProviderDeployment{Type: "serverless", Tier: "workers-ai"}, []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}, catalogs.OfferingRoutabilityRoutable},
		{catalogs.ProviderIDSambaNova, "https://api.sambanova.ai/v1", "", catalogs.ProviderDeployment{Type: "serverless", Tier: "on-demand"}, []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}, catalogs.OfferingRoutabilityRoutable},
		{catalogs.ProviderIDCohere, "https://api.cohere.com", "", catalogs.ProviderDeployment{Type: "serverless"}, []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}, catalogs.OfferingRoutabilityRoutable},
		{catalogs.ProviderIDHuggingFace, "https://router.huggingface.co/v1", "", catalogs.ProviderDeployment{Type: "serverless", Tier: "inference-provider"}, []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}, catalogs.OfferingRoutabilityRoutable},
		{catalogs.ProviderIDNVIDIA, "https://integrate.api.nvidia.com/v1", "", catalogs.ProviderDeployment{Type: "nvidia-hosted", Tier: "developer-catalog"}, []catalogs.InvocationAPI{}, catalogs.OfferingRoutabilityDiscoverable},
		{catalogs.ProviderIDTogetherAI, "https://api.together.ai/v1", "", catalogs.ProviderDeployment{Type: "serverless"}, []catalogs.InvocationAPI{catalogs.InvocationAPIChatCompletions}, catalogs.OfferingRoutabilityRoutable},
	}
	for _, test := range tests {
		t.Run(test.providerID.String(), func(t *testing.T) {
			provider, err := builder.Provider(test.providerID)
			if err != nil {
				t.Fatalf("Provider: %v", err)
			}
			if err := provider.ValidateConfiguration(); err != nil {
				t.Fatalf("ValidateConfiguration: %v", err)
			}
			source := provider.Catalog.Sources[0]
			endpoint := source.Offering.Endpoint
			if endpoint.BaseURL == "" {
				endpoint.BaseURL = strings.TrimSuffix(source.Endpoint.URL, source.Endpoint.Path)
			}
			if endpoint.BaseURL != test.baseURL || endpoint.Path != test.path {
				t.Fatalf("endpoint = %#v, want base=%q path=%q", endpoint, test.baseURL, test.path)
			}
			offering := provider.Catalog.Sources[0].Offering
			if !reflect.DeepEqual(offering.Deployment, test.deployment) {
				t.Fatalf("deployment = %#v, want %#v", offering.Deployment, test.deployment)
			}
			if len(offering.Access.APIs) != len(test.apis) ||
				(len(test.apis) > 0 && !reflect.DeepEqual(offering.Access.APIs, test.apis)) ||
				offering.Access.Routability != test.routability {
				t.Fatalf("access = %#v, want APIs=%#v routability=%q", offering.Access, test.apis, test.routability)
			}
		})
	}
}

func TestConnectorProductionDoesNotImportProviderImplementations(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "connectors", "*", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", file, err)
		}
		if strings.Contains(string(data), "github.com/agentstation/starmap/internal/providers/") {
			t.Fatalf("connector production file imports a provider implementation: %s", file)
		}
	}
}

func TestConnectorTestsOwnProtocolFixturesAndProviderTestsOwnComposition(t *testing.T) {
	connectorTests, err := filepath.Glob(filepath.Join("..", "connectors", "*", "*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range connectorTests {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", file, err)
		}
		text := string(data)
		if strings.Contains(text, "internal/providers/fixtures") || strings.Contains(text, "providers\", \"") && strings.Contains(text, "testdata") {
			t.Fatalf("connector test reaches through the provider fixture seam: %s", file)
		}
	}

	providerTests, err := filepath.Glob(filepath.Join("*", "provider_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range providerTests {
		if filepath.Base(filepath.Dir(file)) == "registry" {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", file, err)
		}
		text := string(data)
		if strings.Contains(text, "openai.NewClient") || strings.Contains(text, "ProviderCatalog{") {
			t.Fatalf("provider integration test bypasses embedded configuration or registry: %s", file)
		}
	}
}

func assertProviderPackageStructure(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir providers: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "fixtures" || entry.Name() == "registry" || entry.Name() == "acquisition" {
			continue
		}
		files, globErr := filepath.Glob(filepath.Join(entry.Name(), "*.go"))
		if globErr != nil {
			t.Fatal(globErr)
		}
		if len(files) == 0 {
			continue
		}
		t.Run("package/"+entry.Name(), func(t *testing.T) {
			recognizedProduction := false
			for _, pair := range [][2]string{{"adapter.go", "adapter_test.go"}, {"client.go", "client_test.go"}, {"source.go", "source_test.go"}, {"pricing.go", "pricing_test.go"}} {
				if _, statErr := os.Stat(filepath.Join(entry.Name(), pair[0])); statErr == nil {
					recognizedProduction = true
					if _, testErr := os.Stat(filepath.Join(entry.Name(), pair[1])); testErr != nil {
						t.Fatalf("%s requires %s: %v", pair[0], pair[1], testErr)
					}
				}
			}
			for _, file := range files {
				if !strings.HasSuffix(file, "_test.go") && !recognizedProduction {
					t.Fatalf("production provider package has no client, adapter, source, or pricing owner: %s", file)
				}
			}
		})
	}
}

func assertConnectorCoverage(t *testing.T, builder *catalogs.Builder) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join("..", "connectors"))
	if err != nil {
		t.Fatalf("ReadDir connectors: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		for _, name := range []string{"client.go", "client_test.go"} {
			if _, err := os.Stat(filepath.Join("..", "connectors", entry.Name(), name)); err != nil {
				t.Fatalf("connector %q requires %s: %v", entry.Name(), name, err)
			}
		}
		if _, err := os.Stat(filepath.Join("..", "connectors", entry.Name(), "response_schema_test.go")); err == nil {
			if _, clientErr := os.Stat(filepath.Join("..", "connectors", entry.Name(), "client_test.go")); clientErr != nil {
				t.Fatal("response_schema_test.go supplements but cannot replace client_test.go")
			}
		}
		endpointTypes := map[string][]catalogs.EndpointType{
			"anthropic": {catalogs.EndpointTypeAnthropic},
			"google":    {catalogs.EndpointTypeGoogle, catalogs.EndpointTypeGoogleCloud},
			"openai":    {catalogs.EndpointTypeOpenAI},
		}[entry.Name()]
		if len(endpointTypes) == 0 {
			t.Fatalf("connector %q has no typed endpoint family", entry.Name())
		}
		used := false
		for _, provider := range builder.Providers().List() {
			for _, source := range provider.Catalog.Sources {
				if slices.Contains(endpointTypes, source.Endpoint.Type) {
					used = true
				}
			}
		}
		if !used {
			t.Fatalf("connector %q has no configured production source", entry.Name())
		}
	}
}

func assertProviderRegistry(t *testing.T) {
	t.Helper()
	for _, name := range []string{"provider.go", "provider_test.go"} {
		if _, err := os.Stat(filepath.Join("registry", name)); err != nil {
			t.Fatalf("provider registry %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join("..", "connectors", "registry")); !os.IsNotExist(err) {
		t.Fatalf("connector registry must not exist: %v", err)
	}
}
