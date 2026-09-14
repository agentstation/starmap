package auth

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

func TestCatalogRoleEnvironmentSelection(t *testing.T) {
	for _, test := range []struct {
		name    string
		product string
		wantErr bool
	}{
		{name: "product precedes conventional", product: "valid-product"},
		{name: "empty product disables fallback", product: "", wantErr: true},
		{name: "invalid product is terminal", product: "malformed", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := ambientCredentialProvider()
			var lookups []string
			resolver := newResolver(func(name string) (string, bool) {
				lookups = append(lookups, name)
				values := map[string]string{
					"STARMAP_OPENAI_API_KEY": test.product,
					"OPENAI_API_KEY":         "valid-conventional",
				}
				value, found := values[name]
				return value, found
			})
			material, err := resolver.ResolveCatalog(t.Context(), &provider)
			if (err != nil) != test.wantErr {
				t.Fatalf("credential selection error = %v, want error %v", err, test.wantErr)
			}
			if !test.wantErr {
				value, _ := material.Value("api-key")
				if value != test.product {
					t.Fatal("catalog acquisition did not select the product credential")
				}
			}
			if len(lookups) != 1 || lookups[0] != "STARMAP_OPENAI_API_KEY" {
				t.Fatalf("credential lookup crossed the selected origin: %v", lookups)
			}
		})
	}
}

func TestCatalogRoleRejectsAliasesBeforeEnvironmentReads(t *testing.T) {
	provider := defaultChainCredentialProvider()
	provider.Credentials.Fields[0].Environment = []string{"SHARED_CREDENTIAL"}
	provider.Credentials.Fields[1].Environment = []string{"SHARED_CREDENTIAL"}
	reads := 0
	resolver := newResolver(func(string) (string, bool) {
		reads++
		return "valid-shared", true
	})
	if _, err := resolver.ResolveCatalog(t.Context(), &provider); err == nil {
		t.Fatal("two credential fields accepted the same environment alias")
	}
	if reads != 0 {
		t.Fatalf("read %d environment values before rejecting alias collision", reads)
	}
}

func TestCatalogRoleExplicitEmptyReferenceDisablesFallback(t *testing.T) {
	provider := ambientCredentialProvider()
	var lookups []string
	resolver := newResolver(func(name string) (string, bool) {
		lookups = append(lookups, name)
		if name == "EXPLICIT_EMPTY" {
			return "", true
		}
		return "valid-ambient", true
	}, WithReferencePolicies(map[CredentialFieldKey]ReferencePolicy{
		{ProviderID: provider.ID, FieldID: "api-key"}: {
			Reference: mustReference(t, "env:EXPLICIT_EMPTY"), FallbackAmbient: true,
		},
	}))
	if _, err := resolver.ResolveCatalog(t.Context(), &provider); err == nil {
		t.Fatal("explicit empty reference selected a fallback credential")
	}
	if len(lookups) != 1 || lookups[0] != "EXPLICIT_EMPTY" {
		t.Fatalf("empty reference read lower candidates: %v", lookups)
	}
}

func TestCatalogRoleKeepsSecretFieldsInOneVersion(t *testing.T) {
	for _, rotate := range []bool{false, true} {
		t.Run(fmt.Sprintf("rotate=%t", rotate), func(t *testing.T) {
			provider := defaultChainCredentialProvider()
			reads := 0
			source := &awsSecretsManagerSource{open: func(context.Context) (awsSecretRead, func() error, error) {
				read := func(_ context.Context, request *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
					reads++
					version := "one"
					if rotate && reads > 1 {
						version = "two"
					}
					if request.VersionId != nil {
						version = *request.VersionId
					}
					return &secretsmanager.GetSecretValueOutput{
						VersionId: aws.String(version),
						SecretString: aws.String(fmt.Sprintf(
							`{"token":"valid-%s","project":"project-%s"}`, version, version)),
					}, nil
				}
				return read, func() error { return nil }, nil
			}}
			resolver := newResolver(mapEnvironment(nil), withCredentialSource(source), WithReferencePolicies(map[CredentialFieldKey]ReferencePolicy{
				{ProviderID: provider.ID, FieldID: "access-token"}: {
					Reference: mustReference(t, "aws-secrets-manager:catalog-profile#token"),
				},
				{ProviderID: provider.ID, FieldID: "project"}: {
					Reference: mustReference(t, "aws-secrets-manager:catalog-profile#project"),
				},
			}))
			material, err := resolver.ResolveCatalog(t.Context(), &provider)
			if err != nil {
				if !rotate {
					t.Fatalf("resolver rejected a stable complete secret: %v", err)
				}
				return
			}
			token, _ := material.Value("access-token")
			project, _ := material.Value("project")
			if strings.TrimPrefix(token, "valid-") != strings.TrimPrefix(project, "project-") {
				t.Fatal("accepted a credential handle assembled from different secret versions")
			}
		})
	}
}
