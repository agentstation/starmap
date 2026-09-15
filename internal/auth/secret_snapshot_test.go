package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
	"testing"

	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	vault "github.com/hashicorp/vault/api"
	openbao "github.com/openbao/openbao/api/v2"

	"github.com/agentstation/starmap/pkg/sources"
)

func TestSecretSnapshotRotatesCompleteProfilesAcrossBackends(t *testing.T) {
	for _, backend := range []ReferenceBackend{
		referenceBackendAWSStore, referenceBackendGCPStore, referenceBackendAzureVault,
		referenceBackendVault, referenceBackendOpenBao,
	} {
		t.Run(string(backend), func(t *testing.T) {
			reads := 0
			source, resource := rotatingProfileSecret(t, backend, &reads)
			provider := defaultChainCredentialProvider()
			resolver := newResolver(mapEnvironment(nil), withCredentialSource(source), WithReferencePolicies(map[CredentialFieldKey]ReferencePolicy{
				{ProviderID: provider.ID, FieldID: "access-token"}: {Reference: mustReference(t, resource+"#token")},
				{ProviderID: provider.ID, FieldID: "project"}:      {Reference: mustReference(t, resource+"#project")},
			}))
			previous := ""
			for version := 1; version <= 2; version++ {
				material, err := resolver.ResolveCatalog(t.Context(), &provider)
				if err != nil {
					t.Fatal(err)
				}
				token, _ := material.Value("access-token")
				project, _ := material.Value("project")
				if token != "token-"+strconv.Itoa(version) || project != "project-"+strconv.Itoa(version) || reads != version {
					t.Fatal("profile did not select one complete response per rotation")
				}
				if material.Version() == previous {
					t.Fatal("rotation retained the old material version")
				}
				previous = material.Version()
				encoded, err := json.Marshal(material.Origins())
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(encoded), "private-deployment") || strings.Contains(string(encoded), token) {
					t.Fatal("origin diagnostics exposed a resource path or credential")
				}
			}
			if len(resolver.cache) != 0 {
				t.Fatal("resolver retained an uncached secret response after profile resolution")
			}
		})
	}
}

func rotatingProfileSecret(t *testing.T, backend ReferenceBackend, reads *int) (credentialSource, string) {
	t.Helper()
	next := func() (string, string, map[string]any) {
		*reads++
		version := strconv.Itoa(*reads)
		data := map[string]any{"token": "token-" + version, "project": "project-" + version}
		payload, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		return version, string(payload), data
	}
	closeClient := func() error { return nil }
	switch backend {
	case referenceBackendAWSStore:
		return &awsSecretsManagerSource{open: func(context.Context) (awsSecretRead, func() error, error) {
			return func(context.Context, *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
				version, payload, _ := next()
				return &secretsmanager.GetSecretValueOutput{VersionId: aws.String(version), SecretString: aws.String(payload)}, nil
			}, closeClient, nil
		}}, "aws-secrets-manager:private-deployment"
	case referenceBackendGCPStore:
		return &gcpSecretManagerSource{open: func(context.Context) (gcpSecretRead, func() error, error) {
			return func(context.Context, string) (*secretmanagerpb.AccessSecretVersionResponse, error) {
				version, payload, _ := next()
				checksum := int64(crc32.Checksum([]byte(payload), crc32.MakeTable(crc32.Castagnoli)))
				return &secretmanagerpb.AccessSecretVersionResponse{
					Name:    "projects/private-deployment/secrets/profile/versions/" + version,
					Payload: &secretmanagerpb.SecretPayload{Data: []byte(payload), DataCrc32C: &checksum},
				}, nil
			}, closeClient, nil
		}}, "gcp-secret-manager:projects/private-deployment/secrets/profile"
	case referenceBackendAzureVault:
		return &azureKeyVaultSource{open: func(string) (azureSecretRead, func() error, error) {
			return func(context.Context, string, string) (azsecrets.GetSecretResponse, error) {
				version, payload, _ := next()
				id := azsecrets.ID("https://private-deployment.vault.azure.net/secrets/profile/" + version)
				return azsecrets.GetSecretResponse{Secret: azsecrets.Secret{Value: &payload, ID: &id}}, nil
			}, closeClient, nil
		}}, "azure-key-vault:https://private-deployment.vault.azure.net/secrets/profile"
	case referenceBackendVault:
		return &vaultSource{open: func() (vaultSecretRead, func() error, error) {
			return func(context.Context, string, string, int) (*vault.KVSecret, error) {
				_, _, data := next()
				return &vault.KVSecret{Data: data, VersionMetadata: &vault.KVVersionMetadata{Version: *reads}}, nil
			}, closeClient, nil
		}}, "vault:secret/private-deployment"
	case referenceBackendOpenBao:
		return &openBaoSource{open: func() (openBaoSecretRead, func() error, error) {
			return func(context.Context, string, string, int) (*openbao.KVSecret, error) {
				_, _, data := next()
				return &openbao.KVSecret{Data: data, VersionMetadata: &openbao.KVVersionMetadata{Version: *reads}}, nil
			}, closeClient, nil
		}}, "openbao:secret/private-deployment"
	default:
		t.Fatalf("unsupported test backend %s", backend)
		return nil, ""
	}
}

func TestSecretSnapshotKeepsScalarAndValueSelectorDistinct(t *testing.T) {
	const payload = `{"value":"selected","project":"project"}`
	for _, first := range []string{"", "value"} {
		material, err := scalarSecretMaterial(referenceBackendAWSStore, []byte(payload), first, "one")
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"", "value"} {
			value, err := referenceValue(material, Reference{backend: referenceBackendAWSStore, field: field})
			want := payload
			if field != "" {
				want = "selected"
			}
			if err != nil || value != want {
				t.Fatal("snapshot confused raw secret bytes with the value field")
			}
		}
	}
}

func TestSecretSnapshotRejectsVersionScopeConflictBeforeReads(t *testing.T) {
	provider := defaultChainCredentialProvider()
	reads := 0
	resolver := newResolver(func(string) (string, bool) { reads++; return "", false }, WithReferencePolicies(map[CredentialFieldKey]ReferencePolicy{
		{ProviderID: provider.ID, FieldID: "access-token"}: {Reference: mustReference(t, "aws-secrets-manager:private-deployment?version=1#token")},
		{ProviderID: provider.ID, FieldID: "project"}:      {Reference: mustReference(t, "aws-secrets-manager:private-deployment?version=2#project")},
	}))
	_, err := resolver.ResolveCatalog(t.Context(), &provider)
	if err == nil || reads != 0 || strings.Contains(err.Error(), "private-deployment") {
		t.Fatalf("version conflict error = %v, ambient reads = %d", err, reads)
	}
}

func TestCredentialOriginsDoNotExposeOrShareMaterial(t *testing.T) {
	provider := ambientCredentialProvider()
	resolver := newResolver(mapEnvironment(map[string]string{"STARMAP_OPENAI_API_KEY": "valid-private-material"}))
	material, err := resolver.ResolveCatalog(t.Context(), &provider)
	if err != nil {
		t.Fatal(err)
	}
	origins := material.Origins()
	want := sources.ProviderCredentialOrigin{Field: "api-key", Kind: "environment", Name: "STARMAP_OPENAI_API_KEY"}
	if len(origins) != 1 || origins[0] != want || material.ResolutionPolicy() != string(EnvironmentPolicyCurrent) {
		t.Fatalf("selected origins = %+v, policy = %s", origins, material.ResolutionPolicy())
	}
	origins[0].Name = "modified"
	if material.Origins()[0] != want {
		t.Fatal("caller mutation changed origin metadata")
	}
	status := NewChecker(WithCredentialResolver(resolver)).CheckProvider(&provider, map[string]bool{string(provider.ID): true})
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{string(encoded), fmt.Sprintf("%+v %#v", material, material)} {
		if strings.Contains(text, "valid-private-material") || strings.Contains(text, material.Version()) {
			t.Fatal("diagnostics exposed credential material or its opaque version")
		}
	}
}
