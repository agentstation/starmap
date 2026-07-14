package catalogs

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestProviderEnvironmentNamesNormalizeScalarAndAliases(t *testing.T) {
	for name, payload := range map[string]string{
		"scalar":  "env: EXAMPLE_API_KEY\n",
		"aliases": "env: [EXAMPLE_API_KEY, EXAMPLE_API_KEY_ALIAS]\n",
	} {
		t.Run(name, func(t *testing.T) {
			var decoded struct {
				Env ProviderEnvironmentNames `yaml:"env"`
			}
			if err := yaml.Unmarshal([]byte(payload), &decoded); err != nil {
				t.Fatalf("decode environment names: %v", err)
			}
			encoded, err := yaml.Marshal(decoded)
			if err != nil {
				t.Fatalf("encode environment names: %v", err)
			}
			var roundTrip struct {
				Env ProviderEnvironmentNames `yaml:"env"`
			}
			if err := yaml.Unmarshal(encoded, &roundTrip); err != nil {
				t.Fatalf("round-trip environment names: %v", err)
			}
			if !reflect.DeepEqual(decoded.Env, roundTrip.Env) {
				t.Fatalf("round trip = %#v, want %#v", roundTrip.Env, decoded.Env)
			}
		})
	}
}

func TestProviderEnvironmentNamesRejectInvalidAliases(t *testing.T) {
	for name, payload := range map[string]string{
		"empty":      "env: []\n",
		"blank":      "env: ['']\n",
		"duplicate":  "env: [EXAMPLE_API_KEY, EXAMPLE_API_KEY]\n",
		"non-string": "env: [EXAMPLE_API_KEY, 7]\n",
	} {
		t.Run(name, func(t *testing.T) {
			var decoded struct {
				Env ProviderEnvironmentNames `yaml:"env"`
			}
			if err := yaml.Unmarshal([]byte(payload), &decoded); err == nil {
				t.Fatal("invalid environment aliases decoded successfully")
			}
		})
	}
}

func TestProviderAuthPolicyNormalizesExactForms(t *testing.T) {
	for name, payload := range map[string]string{
		"none":         "auth: none\n",
		"optional":     "auth: optional\n",
		"required":     "auth: api_key\n",
		"alternatives": "auth: [api_key, cloud_chain]\n",
	} {
		t.Run(name, func(t *testing.T) {
			var decoded struct {
				Auth ProviderAuthPolicy `yaml:"auth" json:"auth"`
			}
			if err := yaml.Unmarshal([]byte(payload), &decoded); err != nil {
				t.Fatalf("decode auth policy: %v", err)
			}
			jsonPayload, err := json.Marshal(decoded)
			if err != nil {
				t.Fatalf("encode auth policy: %v", err)
			}
			var roundTrip struct {
				Auth ProviderAuthPolicy `json:"auth"`
			}
			if err := json.Unmarshal(jsonPayload, &roundTrip); err != nil {
				t.Fatalf("round-trip auth policy: %v", err)
			}
			if !reflect.DeepEqual(decoded.Auth, roundTrip.Auth) {
				t.Fatalf("round trip = %#v, want %#v", roundTrip.Auth, decoded.Auth)
			}
		})
	}
}

func TestProviderAuthPolicyRejectsInvalidLists(t *testing.T) {
	for name, payload := range map[string]string{
		"empty":         "auth: []\n",
		"duplicate":     "auth: [api_key, api_key]\n",
		"none in list":  "auth: [api_key, none]\n",
		"optional list": "auth: [optional, cloud_chain]\n",
	} {
		t.Run(name, func(t *testing.T) {
			var decoded struct {
				Auth ProviderAuthPolicy `yaml:"auth"`
			}
			if err := yaml.Unmarshal([]byte(payload), &decoded); err == nil {
				t.Fatal("invalid auth policy decoded successfully")
			}
		})
	}
}

func TestProviderCredentialNormalization(t *testing.T) {
	normalized, err := (ProviderCredential{Env: ProviderEnvironmentNames{"EXAMPLE_API_KEY"}}).Normalized("api_key")
	if err != nil {
		t.Fatalf("normalize conventional API key: %v", err)
	}
	if normalized.Kind != ProviderCredentialKindAPIKey ||
		normalized.Transport.Header != "Authorization" ||
		normalized.Transport.Scheme != ProviderCredentialSchemeBearer {
		t.Fatalf("normalized API key = %#v", normalized)
	}

	direct, err := (ProviderCredential{
		Env: ProviderEnvironmentNames{"EXAMPLE_API_KEY"},
		Transport: ProviderCredentialTransport{
			Header: "x-api-key", Scheme: ProviderCredentialSchemeDirect,
		},
	}).Normalized("api_key")
	if err != nil {
		t.Fatalf("normalize direct API key: %v", err)
	}
	if direct.Transport.Header != "x-api-key" || direct.Transport.Scheme != ProviderCredentialSchemeDirect {
		t.Fatalf("direct API key = %#v", direct)
	}
}

func TestProviderObservationPolicyNormalizesExactForms(t *testing.T) {
	for name, payload := range map[string]string{
		"invariant": "observation_scope: global_public\n",
		"auth-dependent": `observation_scope:
  anonymous: global_public
  authenticated: credential_scoped
`,
	} {
		t.Run(name, func(t *testing.T) {
			var decoded struct {
				Scope ProviderObservationPolicy `yaml:"observation_scope" json:"observation_scope"`
			}
			if err := yaml.Unmarshal([]byte(payload), &decoded); err != nil {
				t.Fatalf("decode observation policy: %v", err)
			}
			encoded, err := json.Marshal(decoded)
			if err != nil {
				t.Fatalf("encode observation policy: %v", err)
			}
			var roundTrip struct {
				Scope ProviderObservationPolicy `json:"observation_scope"`
			}
			if err := json.Unmarshal(encoded, &roundTrip); err != nil {
				t.Fatalf("round-trip observation policy: %v", err)
			}
			if !reflect.DeepEqual(decoded.Scope, roundTrip.Scope) {
				t.Fatalf("round trip = %#v, want %#v", roundTrip.Scope, decoded.Scope)
			}
		})
	}
}

func TestProviderSourceValidationRejectsInvalidBindingsAndPolicy(t *testing.T) {
	valid := Provider{
		ID: "example", Name: "Example",
		Credentials: map[ProviderCredentialID]ProviderCredential{
			"api_key": {Env: ProviderEnvironmentNames{"EXAMPLE_API_KEY"}},
		},
		Catalog: &ProviderCatalog{Sources: []ProviderSource{{
			ID: "models",
			ObservationScope: ProviderObservationPolicy{
				Invariant: ProviderObservationScopeGlobalPublic,
			},
			Auth: ProviderAuthPolicy{Methods: []ProviderCredentialID{"api_key"}},
			Scopes: map[string]ProviderScopeBinding{
				"region": {
					Source: ProviderBindingSourceEnv,
					Name:   ProviderEnvironmentNames{"EXAMPLE_REGION"},
					Role:   ProviderBindingRoleRequiredInput,
				},
			},
			Endpoint: ProviderSourceEndpoint{Type: EndpointTypeOpenAI, URL: "https://api.example.test/v1/models"},
		}}},
	}

	for name, mutate := range map[string]func(*Provider){
		"unsafe source id": func(provider *Provider) {
			provider.Catalog.Sources[0].ID = "../models"
		},
		"auth-dependent required auth": func(provider *Provider) {
			provider.Catalog.Sources[0].ObservationScope = ProviderObservationPolicy{
				Anonymous: ProviderObservationScopeGlobalPublic, Authenticated: ProviderObservationScopeCredentialScoped,
			}
		},
		"unknown binding source": func(provider *Provider) {
			binding := provider.Catalog.Sources[0].Scopes["region"]
			binding.Source = "ambient"
			provider.Catalog.Sources[0].Scopes["region"] = binding
		},
		"governed sweep as required input": func(provider *Provider) {
			provider.Catalog.Sources[0].Scopes["region"] = ProviderScopeBinding{
				Source: ProviderBindingSourceGovernedSweep, Values: []string{"us-test-1"}, Role: ProviderBindingRoleRequiredInput,
			}
		},
		"unsafe endpoint": func(provider *Provider) {
			provider.Catalog.Sources[0].Endpoint.URL = "http://api.example.test/v1/models"
		},
	} {
		t.Run(name, func(t *testing.T) {
			provider := DeepCopyProvider(valid)
			mutate(&provider)
			if err := provider.ValidateConfiguration(); err == nil {
				t.Fatal("invalid provider configuration validated successfully")
			}
		})
	}
}

func TestTargetProviderContractDeepCopyOwnsNestedConfiguration(t *testing.T) {
	provider := Provider{
		ID: "example", Name: "Example",
		Credentials: map[ProviderCredentialID]ProviderCredential{
			"api_key": {Env: ProviderEnvironmentNames{"EXAMPLE_API_KEY", "EXAMPLE_API_KEY_ALIAS"}},
		},
		Catalog: &ProviderCatalog{Sources: []ProviderSource{{
			ID: "models", ObservationScope: ProviderObservationPolicy{Invariant: ProviderObservationScopeGlobalPublic},
			Auth:     ProviderAuthPolicy{Methods: []ProviderCredentialID{"api_key"}},
			Scopes:   map[string]ProviderScopeBinding{"region": {Source: ProviderBindingSourceEnv, Name: ProviderEnvironmentNames{"EXAMPLE_REGION"}, Role: ProviderBindingRoleRequiredInput}},
			Endpoint: ProviderSourceEndpoint{Type: EndpointTypeOpenAI, URL: "https://api.example.test/v1/models"},
		}}},
	}
	copied := DeepCopyProvider(provider)
	credential := copied.Credentials["api_key"]
	credential.Env[0] = "MUTATED_KEY"
	copied.Credentials["api_key"] = credential
	copied.Catalog.Sources[0].Auth.Methods[0] = "mutated"
	binding := copied.Catalog.Sources[0].Scopes["region"]
	binding.Name[0] = "MUTATED_REGION"
	copied.Catalog.Sources[0].Scopes["region"] = binding
	if provider.Credentials["api_key"].Env[0] != "EXAMPLE_API_KEY" ||
		provider.Catalog.Sources[0].Auth.Methods[0] != "api_key" ||
		provider.Catalog.Sources[0].Scopes["region"].Name[0] != "EXAMPLE_REGION" {
		t.Fatal("deep copy aliased nested provider configuration")
	}
}

func TestTargetProviderContractAcceptsExactSourceShape(t *testing.T) {
	payload := []byte(`
id: example
name: Example
credentials:
  api_key:
    env:
      - EXAMPLE_API_KEY
      - EXAMPLE_API_KEY_ALIAS
catalog:
  sources:
    - id: models
      observation_scope: global_public
      auth: optional
      endpoint:
        type: openai
        url: https://api.example.test/v1/models
invocation:
  routes:
    - id: chat-completions
      api: chat_completions
      auth: api_key
      endpoint: https://api.example.test/v1/chat/completions
`)

	var provider Provider
	if err := yaml.Unmarshal(payload, &provider); err != nil {
		t.Fatalf("decode target provider contract: %v", err)
	}
	if err := provider.ValidateConfiguration(); err != nil {
		t.Fatalf("validate target provider contract: %v", err)
	}
	encoded, err := json.Marshal(provider)
	if err != nil {
		t.Fatalf("encode target provider contract: %v", err)
	}
	var shape map[string]any
	if err := json.Unmarshal(encoded, &shape); err != nil {
		t.Fatalf("decode target JSON shape: %v", err)
	}
	if shape["credentials"] == nil {
		t.Fatal("target credentials were discarded")
	}
	catalog, ok := shape["catalog"].(map[string]any)
	if !ok || catalog["sources"] == nil {
		t.Fatal("target catalog sources were discarded")
	}
	if shape["invocation"] == nil {
		t.Fatal("target invocation routes were discarded")
	}
}

func TestTargetProviderContractRejectsOldSingularShape(t *testing.T) {
	payload := []byte(`
id: old-shape
name: Old Shape
api_key:
  name: OLD_API_KEY
  pattern: .*
  header: Authorization
  scheme: Bearer
  query_param: ""
env_vars:
  - name: OLD_API_KEY
    required: false
catalog:
  endpoint:
    type: openai
    url: https://api.example.test/v1/models
    auth_required: true
`)

	var provider Provider
	if err := yaml.Unmarshal(payload, &provider); err != nil {
		return
	}
	if err := provider.ValidateConfiguration(); err != nil {
		return
	}
	t.Fatal("old singular provider shape remained readable")
}

func TestTargetProviderYAMLDecodeRejectsUnknownAndDuplicateFields(t *testing.T) {
	valid := `
- id: example
  name: Example
  credentials:
    api_key:
      env: EXAMPLE_API_KEY
  catalog:
    sources:
    - id: models
      observation_scope: global_public
      auth: api_key
      endpoint:
        type: openai
        url: https://api.example.test/v1/models
`
	for name, payload := range map[string]string{
		"unknown":         valid + "  unpublished_compatibility: true\n",
		"mixed old field": valid + "  env_vars: []\n",
		"duplicate":       strings.Replace(valid, "  name: Example\n", "  name: Example\n  name: Duplicate\n", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeProvidersYAML([]byte(payload)); err == nil {
				t.Fatal("non-exact provider YAML decoded successfully")
			}
		})
	}
}

func TestTargetProviderContractSupportsOneNamedCompoundMethod(t *testing.T) {
	provider := Provider{
		ID: "compound", Name: "Compound",
		Credentials: map[ProviderCredentialID]ProviderCredential{
			"oauth_client": {
				Kind: ProviderCredentialKindCompound,
				Inputs: map[string]ProviderCredentialInput{
					"client_id":     {Env: ProviderEnvironmentNames{"COMPOUND_CLIENT_ID"}},
					"client_secret": {Env: ProviderEnvironmentNames{"COMPOUND_CLIENT_SECRET"}},
				},
			},
		},
		Catalog: &ProviderCatalog{Sources: []ProviderSource{{
			ID: "models", ObservationScope: ProviderObservationPolicy{Invariant: ProviderObservationScopeCredentialScoped},
			Auth:     ProviderAuthPolicy{Methods: []ProviderCredentialID{"oauth_client"}},
			Endpoint: ProviderSourceEndpoint{Type: EndpointTypeOpenAI, URL: "https://api.example.test/models"},
		}}},
	}
	if err := provider.ValidateConfiguration(); err != nil {
		t.Fatalf("ValidateConfiguration compound: %v", err)
	}
	encoded, err := json.Marshal(provider)
	if err != nil {
		t.Fatalf("Marshal compound: %v", err)
	}
	if strings.Contains(string(encoded), "client-value") || strings.Contains(string(encoded), "secret-value") {
		t.Fatalf("compound metadata encoded a credential value: %s", encoded)
	}
	copyProvider := DeepCopyProvider(provider)
	copyProvider.Credentials["oauth_client"].Inputs["client_id"] = ProviderCredentialInput{Env: ProviderEnvironmentNames{"MUTATED"}}
	if provider.Credentials["oauth_client"].Inputs["client_id"].Env[0] != "COMPOUND_CLIENT_ID" {
		t.Fatal("compound credential inputs aliased through DeepCopyProvider")
	}

	invalid := provider
	invalid.Credentials = map[ProviderCredentialID]ProviderCredential{
		"oauth_client": {Kind: ProviderCredentialKindCompound, Inputs: map[string]ProviderCredentialInput{
			"client_id": {Env: ProviderEnvironmentNames{"COMPOUND_CLIENT_ID"}},
		}},
	}
	if err := invalid.ValidateConfiguration(); err == nil {
		t.Fatal("single-input compound credential validated")
	}
}

func TestProviderConfigurationHasNoRuntimeCredentialValueFields(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeOf(Provider{}), reflect.TypeOf(ProviderCredential{}), reflect.TypeOf(ProviderCredentialInput{}),
	} {
		for index := 0; index < typ.NumField(); index++ {
			field := typ.Field(index)
			name := strings.ToLower(field.Name)
			if name == "value" || strings.Contains(name, "secretvalue") || strings.Contains(name, "tokenvalue") || strings.Contains(name, "session") {
				t.Fatalf("%s contains runtime credential field %s", typ.Name(), field.Name)
			}
		}
	}

	const secret = "runtime-only-secret-value"
	t.Setenv("EXAMPLE_API_KEY", secret)
	provider := Provider{
		ID: "example", Name: "Example",
		Credentials: map[ProviderCredentialID]ProviderCredential{"api_key": {Env: ProviderEnvironmentNames{"EXAMPLE_API_KEY"}}},
		Catalog: &ProviderCatalog{Sources: []ProviderSource{{
			ID: "models", ObservationScope: ProviderObservationPolicy{Invariant: ProviderObservationScopeGlobalPublic},
			Auth:     ProviderAuthPolicy{Methods: []ProviderCredentialID{"api_key"}},
			Endpoint: ProviderSourceEndpoint{Type: EndpointTypeOpenAI, URL: "https://api.example.test/models"},
		}}},
	}
	for name, value := range map[string]Provider{"configured": provider, "copied": DeepCopyProvider(provider)} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("Marshal %s provider: %v", name, err)
		}
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("%s provider encoded runtime secret: %s", name, encoded)
		}
	}
}
