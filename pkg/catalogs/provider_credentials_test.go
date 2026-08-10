package catalogs

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/goccy/go-yaml"
)

func TestCatalogCredentialContract(t *testing.T) {
	t.Parallel()

	openAI := credentialContractProvider()
	if err := openAI.ValidateContract(); err != nil {
		t.Fatalf("ValidateContract: %v", err)
	}

	got, err := DerivedCredentialEnvironmentName("STARPORT", openAI.ID, "api-key")
	if err != nil {
		t.Fatalf("DerivedCredentialEnvironmentName: %v", err)
	}
	if got != "STARPORT_OPENAI_API_KEY" {
		t.Fatalf("derived environment name = %q", got)
	}

	azure := Provider{
		ID:   ProviderIDAzureOpenAI,
		Name: "Azure OpenAI",
		Credentials: &ProviderCredentials{
			Fields: []ProviderCredentialField{
				{ID: "api-key", Kind: ProviderCredentialFieldSecret, Required: true, Environment: []string{"AZURE_OPENAI_API_KEY"}},
				{ID: "access-token", Kind: ProviderCredentialFieldSecret, Required: true},
			},
			Profiles: []ProviderCredentialProfile{
				{
					ID:        "api-key",
					Primitive: ProviderAuthenticationAPIKey,
					Fields:    []ProviderCredentialFieldID{"api-key"},
					Placements: []ProviderCredentialPlacement{{
						Field: "api-key", Kind: ProviderCredentialPlacementHeader,
						Name: "api-key", Scheme: ProviderCredentialSchemeDirect,
					}},
				},
				{
					ID:        "workload-identity",
					Primitive: ProviderAuthenticationAzureDefault,
					Fields:    []ProviderCredentialFieldID{"access-token"},
					Placements: []ProviderCredentialPlacement{{
						Field: "access-token", Kind: ProviderCredentialPlacementHeader,
						Name: "Authorization", Scheme: ProviderCredentialSchemeBearer,
					}},
					Scopes: []string{"https://cognitiveservices.azure.com/.default"},
				},
			},
			CatalogAcquisition: ProviderCredentialPlane{
				Required: true,
				Alternatives: []ProviderCredentialProfileID{
					"api-key", "workload-identity",
				},
			},
			Inference: ProviderCredentialPlane{
				Required: true,
				Alternatives: []ProviderCredentialProfileID{
					"api-key", "workload-identity",
				},
			},
		},
	}
	if err := azure.ValidateContract(); err != nil {
		t.Fatalf("Azure ValidateContract: %v", err)
	}
	if got := azure.Credentials.Inference.Alternatives; !reflect.DeepEqual(got, []ProviderCredentialProfileID{"api-key", "workload-identity"}) {
		t.Fatalf("Azure inference alternatives = %#v", got)
	}

	for _, test := range []struct {
		name   string
		mutate func(*Provider)
		field  string
	}{
		{
			name: "protected header",
			mutate: func(provider *Provider) {
				provider.Credentials.Profiles[0].Placements[0].Name = "Host"
			},
			field: "placements[0].name",
		},
		{
			name: "query without evidence",
			mutate: func(provider *Provider) {
				placement := &provider.Credentials.Profiles[0].Placements[0]
				placement.Kind = ProviderCredentialPlacementQuery
				placement.Name = "key"
				placement.Scheme = ProviderCredentialSchemeDirect
				placement.EvidenceURL = ""
			},
			field: "placements[0].evidence_url",
		},
		{
			name: "unknown profile",
			mutate: func(provider *Provider) {
				provider.Credentials.Inference.Alternatives[0] = "missing"
			},
			field: "inference.alternatives[0]",
		},
		{
			name: "ambiguous binding",
			mutate: func(provider *Provider) {
				profile := &provider.Credentials.Profiles[0]
				profile.EndpointBindings = append(profile.EndpointBindings, profile.EndpointBindings[0])
			},
			field: "endpoint_bindings[1].variable",
		},
		{
			name: "wrong protocol options",
			mutate: func(provider *Provider) {
				provider.Credentials.Profiles[0].ProtocolOptions.AWSDefault = &ProviderAWSDefaultProtocolOptions{
					RegionField: "organization",
					Service:     "bedrock",
				}
			},
			field: "protocol_options.aws_default",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := DeepCopyProvider(openAI)
			test.mutate(&provider)
			err := provider.ValidateContract()
			if err == nil || !strings.Contains(err.Error(), test.field) {
				t.Fatalf("ValidateContract error = %v, want field %q", err, test.field)
			}
		})
	}
}

func TestCatalogCredentialRoundTrip(t *testing.T) {
	t.Parallel()

	want := credentialRoundTripProvider()
	for name, roundTrip := range map[string]func(Provider) (Provider, error){
		"json": func(provider Provider) (Provider, error) {
			encoded, err := json.Marshal(provider)
			if err != nil {
				return Provider{}, err
			}
			var decoded Provider
			decoder := json.NewDecoder(bytes.NewReader(encoded))
			decoder.DisallowUnknownFields()
			err = decoder.Decode(&decoded)
			return decoded, err
		},
		"yaml": func(provider Provider) (Provider, error) {
			encoded, err := yaml.Marshal(provider)
			if err != nil {
				return Provider{}, err
			}
			var decoded Provider
			err = yaml.UnmarshalWithOptions(encoded, &decoded, yaml.Strict())
			return decoded, err
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := roundTrip(want)
			if err != nil {
				t.Fatalf("round trip: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("round trip mismatch\ngot:  %#v\nwant: %#v", got, want)
			}
		})
	}

	var decoded Provider
	unknown := []byte(`{"id":"provider","name":"Provider","credentials":{"fields":[],"profiles":[],"catalog_acquisition":{},"inference":{},"fallback":true}}`)
	decoder := json.NewDecoder(bytes.NewReader(unknown))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err == nil {
		t.Fatal("strict JSON accepted an unknown credential field")
	}
	if err := yaml.UnmarshalWithOptions([]byte("id: provider\nname: Provider\ncredentials:\n  fallback: true\n"), &decoded, yaml.Strict()); err == nil {
		t.Fatal("strict YAML accepted an unknown credential field")
	}
}

func TestCatalogCredentialCopyIsolation(t *testing.T) {
	t.Parallel()

	original := credentialRoundTripProvider()
	copied := DeepCopyProvider(original)
	copied.Credentials.Fields[0].Environment[0] = "CHANGED_API_KEY"
	copied.Credentials.Profiles[0].Fields[0] = "organization"
	copied.Credentials.Profiles[0].Placements[0].Name = "X-Changed"
	copied.Credentials.Profiles[1].Scopes[0] = "changed-scope"
	copied.Credentials.Profiles[0].EndpointBindings[0].Variable = "changed"
	copied.Credentials.Profiles[1].ProtocolOptions.GoogleDefault.QuotaProjectField = "api-key"
	copied.Credentials.CatalogAcquisition.Alternatives[0] = "changed"
	copied.Credentials.Inference.Alternatives[0] = "changed"

	profile := original.Credentials.Profiles[0]
	if original.Credentials.Fields[0].Environment[0] != "OPENAI_API_KEY" ||
		profile.Fields[0] != "api-key" ||
		profile.Placements[0].Name != "Authorization" ||
		original.Credentials.Profiles[1].Scopes[0] != "https://example.test/scope" ||
		profile.EndpointBindings[0].Variable != "organization" ||
		original.Credentials.Profiles[1].ProtocolOptions.GoogleDefault.QuotaProjectField != "organization" ||
		original.Credentials.CatalogAcquisition.Alternatives[0] != "default" ||
		original.Credentials.Inference.Alternatives[0] != "default" {
		t.Fatal("credential copy shares nested mutable state")
	}
}

func TestCatalogCredentialPlanesAreIsolated(t *testing.T) {
	t.Parallel()

	provider := credentialContractProvider()
	provider.Credentials.CatalogAcquisition.Alternatives[0] = "catalog-only"
	if got := provider.Credentials.Inference.Alternatives[0]; got != "default" {
		t.Fatalf("inference alternative changed to %q", got)
	}
}

func TestCatalogCredentialsNeverSerializeValues(t *testing.T) {
	provider := credentialContractProvider()
	provider.apiKeyValue = "catalog-credential-secret"
	provider.EnvVarValues = map[string]string{"OPENAI_API_KEY": "environment-credential-secret"}

	for name, marshal := range map[string]func(any) ([]byte, error){
		"json": json.Marshal,
		"yaml": yaml.Marshal,
	} {
		t.Run(name, func(t *testing.T) {
			encoded, err := marshal(provider)
			if err != nil {
				t.Fatalf("marshal provider: %v", err)
			}
			for _, secret := range []string{"catalog-credential-secret", "environment-credential-secret"} {
				if strings.Contains(string(encoded), secret) {
					t.Fatalf("serialized provider contains credential value %q", secret)
				}
			}
		})
	}
}

func TestCredentialAliasCollisionsFailBeforeEnvironmentReads(t *testing.T) {
	t.Setenv("STARPORT_OPENAI_API_KEY", "must-not-affect-validation")

	for _, test := range []struct {
		name      string
		providers []Provider
	}{
		{
			name: "derived alias",
			providers: []Provider{
				credentialProviderWithField("openai", "api-key", "OPENAI_API_KEY"),
				credentialProviderWithField("openai-api", "key", "OPENAI_COMPATIBLE_KEY"),
			},
		},
		{
			name: "conventional environment",
			providers: []Provider{
				credentialProviderWithField("one", "api-key", "SHARED_API_KEY"),
				credentialProviderWithField("two", "api-key", "SHARED_API_KEY"),
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			builder := NewEmpty()
			for _, provider := range test.providers {
				if err := builder.SetProvider(provider); err != nil {
					t.Fatalf("SetProvider: %v", err)
				}
			}
			if _, err := NewObservationCatalog(builder); err == nil {
				t.Fatal("catalog accepted a credential environment collision")
			}
		})
	}
}

func TestCatalogCredentialValidationRejectsInvalidContracts(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		mutate func(*Provider)
		field  string
	}{
		{
			name: "duplicate field ID",
			mutate: func(provider *Provider) {
				provider.Credentials.Fields = append(provider.Credentials.Fields, provider.Credentials.Fields[0])
			},
			field: "fields[2].id",
		},
		{
			name: "duplicate profile ID",
			mutate: func(provider *Provider) {
				provider.Credentials.Profiles = append(provider.Credentials.Profiles, provider.Credentials.Profiles[0])
			},
			field: "profiles[1].id",
		},
		{
			name: "duplicate profile field",
			mutate: func(provider *Provider) {
				provider.Credentials.Profiles[0].Fields = append(
					provider.Credentials.Profiles[0].Fields,
					"api-key",
				)
			},
			field: "profiles[0].fields[2]",
		},
		{
			name: "invalid conventional environment",
			mutate: func(provider *Provider) {
				provider.Credentials.Fields[0].Environment[0] = "openai-api-key"
			},
			field: "fields[0].environment[0]",
		},
		{
			name: "invalid field pattern",
			mutate: func(provider *Provider) {
				provider.Credentials.Fields[0].Pattern = "["
			},
			field: "fields[0].pattern",
		},
		{
			name: "secret endpoint binding",
			mutate: func(provider *Provider) {
				provider.Credentials.Profiles[0].EndpointBindings[0].Field = "api-key"
			},
			field: "endpoint_bindings[0].field",
		},
		{
			name: "duplicate plane alternative",
			mutate: func(provider *Provider) {
				provider.Credentials.Inference.Alternatives = append(
					provider.Credentials.Inference.Alternatives,
					"default",
				)
			},
			field: "inference.alternatives[1]",
		},
		{
			name: "unsupported primitive",
			mutate: func(provider *Provider) {
				provider.Credentials.Profiles[0].Primitive = "provider-specific"
			},
			field: "profiles[0].primitive",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := credentialContractProvider()
			test.mutate(&provider)
			err := provider.ValidateContract()
			if err == nil || !strings.Contains(err.Error(), test.field) {
				t.Fatalf("ValidateContract error = %v, want field %q", err, test.field)
			}
		})
	}
}

func TestCatalogCredentialOptionalNoneProfile(t *testing.T) {
	t.Parallel()

	provider := Provider{
		ID: "local", Name: "Local",
		Credentials: &ProviderCredentials{
			Profiles: []ProviderCredentialProfile{{
				ID: "unauthenticated", Primitive: ProviderAuthenticationNone,
			}},
			CatalogAcquisition: ProviderCredentialPlane{
				Alternatives: []ProviderCredentialProfileID{"unauthenticated"},
			},
			Inference: ProviderCredentialPlane{
				Alternatives: []ProviderCredentialProfileID{"unauthenticated"},
			},
		},
	}
	if err := provider.ValidateContract(); err != nil {
		t.Fatalf("ValidateContract: %v", err)
	}

	provider.Credentials.Inference.Required = true
	if err := provider.ValidateContract(); err == nil || !strings.Contains(err.Error(), "inference.alternatives[0]") {
		t.Fatalf("required unauthenticated plane error = %v", err)
	}
}

func TestProviderCredentialYAMLIsStrict(t *testing.T) {
	t.Parallel()

	_, err := New(WithFS(fstest.MapFS{
		"providers.yaml": &fstest.MapFile{Data: []byte(`
- id: provider
  name: Provider
  credentials:
    fields: []
    profiles: []
    catalog_acquisition: {}
    inference: {}
    fallback: true
`)},
	}))
	if err == nil || !strings.Contains(err.Error(), "fallback") {
		t.Fatalf("strict provider YAML error = %v", err)
	}
}

func credentialContractProvider() Provider {
	return Provider{
		ID:   ProviderIDOpenAI,
		Name: "OpenAI",
		Credentials: &ProviderCredentials{
			Fields: []ProviderCredentialField{
				{
					ID: "api-key", Kind: ProviderCredentialFieldSecret, Required: true,
					Environment: []string{"OPENAI_API_KEY"}, Pattern: `^sk-`,
					Description: "OpenAI API key",
				},
				{
					ID: "organization", Kind: ProviderCredentialFieldParameter,
					Environment: []string{"OPENAI_ORG_ID"}, Description: "Optional organization",
				},
			},
			Profiles: []ProviderCredentialProfile{{
				ID:        "default",
				Primitive: ProviderAuthenticationAPIKey,
				Fields:    []ProviderCredentialFieldID{"api-key", "organization"},
				Placements: []ProviderCredentialPlacement{
					{Field: "api-key", Kind: ProviderCredentialPlacementHeader, Name: "Authorization", Scheme: ProviderCredentialSchemeBearer},
					{Field: "organization", Kind: ProviderCredentialPlacementHeader, Name: "OpenAI-Organization", Scheme: ProviderCredentialSchemeDirect},
				},
				EndpointBindings: []ProviderCredentialEndpointBinding{{
					Field: "organization", Variable: "organization",
				}},
			}},
			CatalogAcquisition: ProviderCredentialPlane{Required: true, Alternatives: []ProviderCredentialProfileID{"default"}},
			Inference:          ProviderCredentialPlane{Required: true, Alternatives: []ProviderCredentialProfileID{"default"}},
		},
	}
}

func credentialRoundTripProvider() Provider {
	provider := credentialContractProvider()
	provider.ID = "contract-provider"
	provider.Name = "Contract Provider"
	provider.Credentials.Fields = append(provider.Credentials.Fields, ProviderCredentialField{
		ID: "access-token", Kind: ProviderCredentialFieldSecret, Required: true,
	})
	provider.Credentials.Profiles = append(provider.Credentials.Profiles, ProviderCredentialProfile{
		ID:        "google-workload",
		Primitive: ProviderAuthenticationGoogleDefault,
		Fields:    []ProviderCredentialFieldID{"access-token", "organization"},
		Placements: []ProviderCredentialPlacement{{
			Field: "access-token", Kind: ProviderCredentialPlacementHeader,
			Name: "Authorization", Scheme: ProviderCredentialSchemeBearer,
		}},
		Scopes: []string{"https://example.test/scope"},
		EndpointBindings: []ProviderCredentialEndpointBinding{{
			Field: "organization", Variable: "organization",
		}},
		ProtocolOptions: ProviderAuthenticationProtocolOptions{
			GoogleDefault: &ProviderGoogleDefaultProtocolOptions{QuotaProjectField: "organization"},
		},
	})
	provider.Credentials.Inference.Alternatives = append(
		provider.Credentials.Inference.Alternatives,
		"google-workload",
	)
	return provider
}

func credentialProviderWithField(providerID ProviderID, fieldID ProviderCredentialFieldID, environment string) Provider {
	return Provider{
		ID: providerID, Name: string(providerID),
		Credentials: &ProviderCredentials{
			Fields: []ProviderCredentialField{{
				ID: fieldID, Kind: ProviderCredentialFieldSecret, Required: true,
				Environment: []string{environment},
			}},
			Profiles: []ProviderCredentialProfile{{
				ID: "default", Primitive: ProviderAuthenticationAPIKey,
				Fields: []ProviderCredentialFieldID{fieldID},
				Placements: []ProviderCredentialPlacement{{
					Field: fieldID, Kind: ProviderCredentialPlacementHeader,
					Name: "Authorization", Scheme: ProviderCredentialSchemeBearer,
				}},
			}},
			Inference: ProviderCredentialPlane{Required: true, Alternatives: []ProviderCredentialProfileID{"default"}},
		},
	}
}
