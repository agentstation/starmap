package auth

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	cloudauth "cloud.google.com/go/auth"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	azurepolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/agentstation/starmap/pkg/catalogs"
)

type fakeGoogleCredentials struct {
	token        func(context.Context) (*cloudauth.Token, error)
	project      string
	quotaProject string
}

func (c fakeGoogleCredentials) Token(ctx context.Context) (*cloudauth.Token, error) {
	return c.token(ctx)
}

func (c fakeGoogleCredentials) ProjectID(context.Context) (string, error) {
	return c.project, nil
}

func (c fakeGoogleCredentials) QuotaProjectID(context.Context) (string, error) {
	return c.quotaProject, nil
}

func TestGoogleDefaultChainProjectsTokenAndProjectLifecycle(t *testing.T) {
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	chain := &googleDefaultChain{
		now: func() time.Time { return now },
		detect: func(scopes []string) (googleCredentials, error) {
			if len(scopes) != 1 || scopes[0] != "https://example.test/scope" {
				t.Fatalf("scopes = %v", scopes)
			}
			return fakeGoogleCredentials{
				token: func(context.Context) (*cloudauth.Token, error) {
					return &cloudauth.Token{Value: "google-token", Expiry: expiresAt}, nil
				},
				project: "project-from-credentials",
			}, nil
		},
	}
	profile := defaultChainCredentialProvider().Credentials.Profiles[0]

	material, err := chain.resolve(context.Background(), profile, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got := material.values["access-token"]; got != "google-token" {
		t.Fatalf("access token = %q", got)
	}
	if got := material.values["project"]; got != "project-from-credentials" {
		t.Fatalf("project = %q", got)
	}
	if !material.expiresAt.Equal(expiresAt) || material.lease == nil ||
		!material.lease.RefreshAfter.Equal(expiresAt.Add(-credentialRefreshSkew)) {
		t.Fatalf("lifecycle = expiry %v, lease %#v", material.expiresAt, material.lease)
	}
}

type fakeAzureCredential struct {
	getToken func(context.Context, azurepolicy.TokenRequestOptions) (azcore.AccessToken, error)
}

func (c fakeAzureCredential) GetToken(
	ctx context.Context,
	options azurepolicy.TokenRequestOptions,
) (azcore.AccessToken, error) {
	return c.getToken(ctx, options)
}

func TestAzureDefaultChainProjectsTokenLifecycle(t *testing.T) {
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	chain := &azureDefaultChain{
		now: func() time.Time { return now },
		newCredential: func() (azureTokenCredential, error) {
			return fakeAzureCredential{getToken: func(
				_ context.Context,
				options azurepolicy.TokenRequestOptions,
			) (azcore.AccessToken, error) {
				if len(options.Scopes) != 1 || options.Scopes[0] != "https://example.test/.default" {
					t.Fatalf("scopes = %v", options.Scopes)
				}
				return azcore.AccessToken{Token: "azure-token", ExpiresOn: expiresAt}, nil
			}}, nil
		},
	}
	profile := bearerCloudProfile(catalogs.ProviderAuthenticationAzureDefault)

	material, err := chain.resolve(context.Background(), profile, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got := material.values["access-token"]; got != "azure-token" {
		t.Fatalf("access token = %q", got)
	}
	if !material.expiresAt.Equal(expiresAt) || material.lease == nil ||
		!material.lease.Renewable ||
		!material.lease.RefreshAfter.Equal(expiresAt.Add(-credentialRefreshSkew)) {
		t.Fatalf("lifecycle = expiry %v, lease %#v", material.expiresAt, material.lease)
	}
}

func TestAzureDefaultChainPropagatesCancellation(t *testing.T) {
	chain := &azureDefaultChain{
		now: time.Now,
		newCredential: func() (azureTokenCredential, error) {
			return fakeAzureCredential{getToken: func(
				ctx context.Context,
				_ azurepolicy.TokenRequestOptions,
			) (azcore.AccessToken, error) {
				<-ctx.Done()
				return azcore.AccessToken{}, ctx.Err()
			}}, nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := chain.resolve(ctx, bearerCloudProfile(catalogs.ProviderAuthenticationAzureDefault), nil)
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("resolve error = %v", err)
	}
}

func TestAWSDefaultChainProjectsCredentialLifecycle(t *testing.T) {
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	chain := &awsDefaultChain{
		now: func() time.Time { return now },
		load: func(context.Context) (aws.Config, error) {
			return aws.Config{
				Region: "us-east-1",
				Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
					return aws.Credentials{
						AccessKeyID: "access-key", SecretAccessKey: "secret-key",
						SessionToken: "session-token", CanExpire: true, Expires: expiresAt,
					}, nil
				}),
			}, nil
		},
	}
	profile := catalogs.ProviderCredentialProfile{
		ID: "workload-identity", Primitive: catalogs.ProviderAuthenticationAWSDefault,
		Fields: []catalogs.ProviderCredentialFieldID{
			catalogs.ProviderAWSCredentialAccessKeyID,
			catalogs.ProviderAWSCredentialSecretAccessKey,
			catalogs.ProviderAWSCredentialSessionToken,
			"region",
		},
		ProtocolOptions: catalogs.ProviderAuthenticationProtocolOptions{
			AWSDefault: &catalogs.ProviderAWSDefaultProtocolOptions{
				RegionField: "region", Service: "bedrock",
			},
		},
	}

	material, err := chain.resolve(context.Background(), profile, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	want := map[string]string{
		"access-key-id": "access-key", "secret-access-key": "secret-key",
		"session-token": "session-token", "region": "us-east-1",
	}
	for field, value := range want {
		if got := material.values[field]; got != value {
			t.Fatalf("field %s = %q, want %q", field, got, value)
		}
	}
	if !material.expiresAt.Equal(expiresAt) || material.lease == nil ||
		!material.lease.RefreshAfter.Equal(expiresAt.Add(-credentialRefreshSkew)) {
		t.Fatalf("lifecycle = expiry %v, lease %#v", material.expiresAt, material.lease)
	}
}

func bearerCloudProfile(
	primitive catalogs.ProviderAuthenticationPrimitive,
) catalogs.ProviderCredentialProfile {
	return catalogs.ProviderCredentialProfile{
		ID: "workload-identity", Primitive: primitive,
		Fields: []catalogs.ProviderCredentialFieldID{"access-token"},
		Placements: []catalogs.ProviderCredentialPlacement{{
			Field: "access-token", Kind: catalogs.ProviderCredentialPlacementHeader,
			Name: "Authorization", Scheme: catalogs.ProviderCredentialSchemeBearer,
		}},
		Scopes: []string{"https://example.test/.default"},
	}
}
