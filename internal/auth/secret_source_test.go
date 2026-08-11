package auth

import (
	"context"
	"hash/crc32"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/smithy-go"
	vault "github.com/hashicorp/vault/api"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDefaultDirectSecretSourcesAreRegistered(t *testing.T) {
	want := []ReferenceBackend{
		referenceBackendAWSStore,
		referenceBackendAzureVault,
		referenceBackendGCPStore,
		referenceBackendVault,
	}
	resolver := newResolver(mapEnvironment(nil))
	got := make([]ReferenceBackend, 0, len(want))
	for _, backend := range want {
		if _, exists := resolver.sources[backend]; !exists {
			t.Fatalf("source %q is not registered", backend)
		}
		got = append(got, backend)
	}
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	if strings.Join(referenceBackends(got), ",") != strings.Join(referenceBackends(want), ",") {
		t.Fatalf("backends = %v, want %v", got, want)
	}
}

func TestGCPSecretManagerSourceResolvesVersionedJSONFieldAndCloses(t *testing.T) {
	const payload = `{"api-key":"valid-gcp","other":"preserved"}`
	checksum := int64(crc32.Checksum([]byte(payload), crc32.MakeTable(crc32.Castagnoli)))
	var closed atomic.Int32
	source := &gcpSecretManagerSource{open: func(context.Context) (gcpSecretRead, func() error, error) {
		read := func(_ context.Context, name string) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			if name != "projects/project/secrets/provider-key/versions/7" {
				t.Fatalf("name = %q", name)
			}
			return &secretmanagerpb.AccessSecretVersionResponse{
				Name: name,
				Payload: &secretmanagerpb.SecretPayload{
					Data: []byte(payload), DataCrc32C: &checksum,
				},
			}, nil
		}
		return read, func() error { closed.Add(1); return nil }, nil
	}}

	material, err := source.Resolve(
		context.Background(),
		mustReference(t, "gcp-secret-manager:projects/project/secrets/provider-key?version=7#api-key"),
	)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := material.values["api-key"]; got != "valid-gcp" {
		t.Fatalf("api-key = %q", got)
	}
	if material.version != "projects/project/secrets/provider-key/versions/7" {
		t.Fatalf("version = %q", material.version)
	}
	if closed.Load() != 1 {
		t.Fatalf("close calls = %d, want 1", closed.Load())
	}
}

func TestGCPSecretManagerSourceValidatesReferenceChecksumAndErrors(t *testing.T) {
	opened := false
	source := &gcpSecretManagerSource{open: func(context.Context) (gcpSecretRead, func() error, error) {
		opened = true
		return func(context.Context, string) (*secretmanagerpb.AccessSecretVersionResponse, error) {
			badChecksum := int64(1)
			return &secretmanagerpb.AccessSecretVersionResponse{
				Name: "projects/project/secrets/key/versions/1",
				Payload: &secretmanagerpb.SecretPayload{
					Data: []byte("value"), DataCrc32C: &badChecksum,
				},
			}, nil
		}, func() error { return nil }, nil
	}}
	_, err := source.Resolve(context.Background(), mustReference(t, "gcp-secret-manager:invalid"))
	if !isSourceError(err, SourceErrorInvalid) || opened {
		t.Fatalf("invalid reference error = %v, opened = %t", err, opened)
	}
	_, err = source.Resolve(
		context.Background(),
		mustReference(t, "gcp-secret-manager:projects/project/secrets/key"),
	)
	if !isSourceError(err, SourceErrorInvalid) {
		t.Fatalf("checksum error = %v", err)
	}

	for _, test := range []struct {
		code codes.Code
		kind SourceErrorKind
	}{
		{code: codes.NotFound, kind: SourceErrorNotConfigured},
		{code: codes.PermissionDenied, kind: SourceErrorDenied},
		{code: codes.InvalidArgument, kind: SourceErrorInvalid},
		{code: codes.Unavailable, kind: SourceErrorUnavailable},
	} {
		if got := classifyGCPSecretError(status.Error(test.code, "sensitive")); got != test.kind {
			t.Fatalf("code %s = %s, want %s", test.code, got, test.kind)
		}
	}
}

func TestAzureKeyVaultSourceResolvesVersionedJSONFieldAndCloses(t *testing.T) {
	var closed atomic.Int32
	source := &azureKeyVaultSource{open: func(vaultURL string) (azureSecretRead, func() error, error) {
		if vaultURL != "https://catalog.vault.azure.net" {
			t.Fatalf("vault URL = %q", vaultURL)
		}
		read := func(_ context.Context, name, version string) (azsecrets.GetSecretResponse, error) {
			if name != "provider-key" || version != "7" {
				t.Fatalf("name, version = %q, %q", name, version)
			}
			value := `{"api-key":"valid-azure"}`
			id := azsecrets.ID("https://catalog.vault.azure.net/secrets/provider-key/7")
			return azsecrets.GetSecretResponse{Secret: azsecrets.Secret{Value: &value, ID: &id}}, nil
		}
		return read, func() error { closed.Add(1); return nil }, nil
	}}

	material, err := source.Resolve(
		context.Background(),
		mustReference(t, "azure-key-vault:https://catalog.vault.azure.net/secrets/provider-key?version=7#api-key"),
	)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if material.values["api-key"] != "valid-azure" || material.version != "7" {
		t.Fatalf("material = %#v", material)
	}
	if closed.Load() != 1 {
		t.Fatalf("close calls = %d, want 1", closed.Load())
	}
}

func TestAzureKeyVaultSourceRejectsUnsafeResourceAndClassifiesErrors(t *testing.T) {
	source := &azureKeyVaultSource{open: func(string) (azureSecretRead, func() error, error) {
		t.Fatal("invalid resource opened a client")
		return nil, nil, nil
	}}
	for _, value := range []string{
		"azure-key-vault:http://vault.test/secrets/key",
		"azure-key-vault:https://user@vault.test/secrets/key",
		"azure-key-vault:https://vault.test/secrets/key/extra",
	} {
		_, err := source.Resolve(context.Background(), mustReference(t, value))
		if !isSourceError(err, SourceErrorInvalid) {
			t.Fatalf("Resolve(%q) error = %v", value, err)
		}
	}
	for _, test := range []struct {
		status int
		kind   SourceErrorKind
	}{
		{status: 404, kind: SourceErrorNotConfigured},
		{status: 403, kind: SourceErrorDenied},
		{status: 400, kind: SourceErrorInvalid},
		{status: 503, kind: SourceErrorUnavailable},
	} {
		err := &azcore.ResponseError{StatusCode: test.status}
		if got := classifyAzureSecretError(err); got != test.kind {
			t.Fatalf("status %d = %s, want %s", test.status, got, test.kind)
		}
	}
}

func TestAWSSecretsManagerSourceResolvesVersionedJSONFieldAndCloses(t *testing.T) {
	var closed atomic.Int32
	source := &awsSecretsManagerSource{open: func(context.Context) (awsSecretRead, func() error, error) {
		read := func(
			_ context.Context,
			input *secretsmanager.GetSecretValueInput,
		) (*secretsmanager.GetSecretValueOutput, error) {
			if aws.ToString(input.SecretId) != "provider/key" || aws.ToString(input.VersionId) != "version-7" {
				t.Fatalf("input = %#v", input)
			}
			value := `{"api-key":"valid-aws"}`
			return &secretsmanager.GetSecretValueOutput{
				SecretString: &value, VersionId: aws.String("version-7"),
			}, nil
		}
		return read, func() error { closed.Add(1); return nil }, nil
	}}
	material, err := source.Resolve(
		context.Background(),
		mustReference(t, "aws-secrets-manager:provider/key?version=version-7#api-key"),
	)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if material.values["api-key"] != "valid-aws" || material.version != "version-7" {
		t.Fatalf("material = %#v", material)
	}
	if closed.Load() != 1 {
		t.Fatalf("close calls = %d, want 1", closed.Load())
	}
}

func TestAWSSecretsManagerSourceSupportsExactBinaryBytesAndClassifiesErrors(t *testing.T) {
	source := &awsSecretsManagerSource{open: func(context.Context) (awsSecretRead, func() error, error) {
		read := func(context.Context, *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
			return &secretsmanager.GetSecretValueOutput{
				SecretBinary: []byte{'a', 0, 'b'}, VersionId: aws.String("1"),
			}, nil
		}
		return read, func() error { return nil }, nil
	}}
	material, err := source.Resolve(
		context.Background(), mustReference(t, "aws-secrets-manager:provider/key"),
	)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := material.values["value"]; got != string([]byte{'a', 0, 'b'}) {
		t.Fatalf("value bytes = %q", got)
	}

	for _, test := range []struct {
		code string
		kind SourceErrorKind
	}{
		{code: "ResourceNotFoundException", kind: SourceErrorNotConfigured},
		{code: "AccessDeniedException", kind: SourceErrorDenied},
		{code: "InvalidParameterException", kind: SourceErrorInvalid},
		{code: "ThrottlingException", kind: SourceErrorUnavailable},
	} {
		err := &smithy.GenericAPIError{Code: test.code, Message: "sensitive"}
		if got := classifyAWSSecretError(err); got != test.kind {
			t.Fatalf("code %s = %s, want %s", test.code, got, test.kind)
		}
	}
}

func TestVaultSourceResolvesVersionedFieldAndCloses(t *testing.T) {
	var closed atomic.Int32
	source := &vaultSource{open: func() (vaultSecretRead, func() error, error) {
		read := func(_ context.Context, mount, path string, version int) (*vault.KVSecret, error) {
			if mount != "secret" || path != "apps/catalog" || version != 7 {
				t.Fatalf("read = %q, %q, %d", mount, path, version)
			}
			return &vault.KVSecret{
				Data:            map[string]any{"api-key": "valid-vault", "other": "preserved"},
				VersionMetadata: &vault.KVVersionMetadata{Version: 7},
			}, nil
		}
		return read, func() error { closed.Add(1); return nil }, nil
	}}
	material, err := source.Resolve(
		context.Background(), mustReference(t, "vault:secret/apps/catalog?version=7#api-key"),
	)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if material.values["api-key"] != "valid-vault" || material.version != "7" {
		t.Fatalf("material = %#v", material)
	}
	if closed.Load() != 1 {
		t.Fatalf("close calls = %d, want 1", closed.Load())
	}
}

func TestVaultSourceValidatesReferenceDataAndErrors(t *testing.T) {
	for _, value := range []string{
		"vault:secret", "vault:secret/../key", "vault:secret/key?version=0",
	} {
		source := &vaultSource{open: func() (vaultSecretRead, func() error, error) {
			t.Fatal("invalid reference opened a client")
			return nil, nil, nil
		}}
		_, err := source.Resolve(context.Background(), mustReference(t, value))
		if !isSourceError(err, SourceErrorInvalid) {
			t.Fatalf("Resolve(%q) error = %v", value, err)
		}
	}

	_, err := vaultSourceMaterial(referenceBackendVault, &vault.KVSecret{
		Data: map[string]any{"api-key": 7}, VersionMetadata: &vault.KVVersionMetadata{Version: 1},
	}, "api-key")
	if !isSourceError(err, SourceErrorInvalid) {
		t.Fatalf("non-string field error = %v", err)
	}
	for _, test := range []struct {
		status int
		kind   SourceErrorKind
	}{
		{status: 404, kind: SourceErrorNotConfigured},
		{status: 403, kind: SourceErrorDenied},
		{status: 400, kind: SourceErrorInvalid},
		{status: 503, kind: SourceErrorUnavailable},
	} {
		err := &vault.ResponseError{StatusCode: test.status}
		if got := classifyVaultSecretError(err); got != test.kind {
			t.Fatalf("status %d = %s, want %s", test.status, got, test.kind)
		}
	}
}

func TestDirectSecretSourcesPropagateCancellationWithinBudget(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sources := []struct {
		name      string
		reference string
		resolve   func(context.Context, Reference) (sourceMaterial, error)
	}{
		{
			name: "gcp", reference: "gcp-secret-manager:projects/project/secrets/key",
			resolve: (&gcpSecretManagerSource{open: func(context.Context) (gcpSecretRead, func() error, error) {
				return func(readCtx context.Context, _ string) (*secretmanagerpb.AccessSecretVersionResponse, error) {
					<-readCtx.Done()
					return nil, readCtx.Err()
				}, func() error { return nil }, nil
			}}).Resolve,
		},
		{
			name: "azure", reference: "azure-key-vault:https://vault.test/secrets/key",
			resolve: (&azureKeyVaultSource{open: func(string) (azureSecretRead, func() error, error) {
				return func(readCtx context.Context, _, _ string) (azsecrets.GetSecretResponse, error) {
					<-readCtx.Done()
					return azsecrets.GetSecretResponse{}, readCtx.Err()
				}, func() error { return nil }, nil
			}}).Resolve,
		},
		{
			name: "aws", reference: "aws-secrets-manager:key",
			resolve: (&awsSecretsManagerSource{open: func(context.Context) (awsSecretRead, func() error, error) {
				return func(readCtx context.Context, _ *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
					<-readCtx.Done()
					return nil, readCtx.Err()
				}, func() error { return nil }, nil
			}}).Resolve,
		},
		{
			name: "vault", reference: "vault:secret/key",
			resolve: (&vaultSource{open: func() (vaultSecretRead, func() error, error) {
				return func(readCtx context.Context, _, _ string, _ int) (*vault.KVSecret, error) {
					<-readCtx.Done()
					return nil, readCtx.Err()
				}, func() error { return nil }, nil
			}}).Resolve,
		},
	}
	for _, source := range sources {
		t.Run(source.name, func(t *testing.T) {
			started := time.Now()
			_, err := source.resolve(ctx, mustReference(t, source.reference))
			if err != context.Canceled {
				t.Fatalf("Resolve error = %v", err)
			}
			if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
				t.Fatalf("cancellation took %s", elapsed)
			} else {
				t.Logf("cancellation = %s", elapsed)
			}
		})
	}
}

func TestDirectSecretSourceColdFakeP95Budget(t *testing.T) {
	sources := directSecretPerformanceSources(t)
	const calls = 10_000
	durations := make([]time.Duration, 0, calls)
	for index := range calls {
		source := sources[index%len(sources)]
		started := time.Now()
		if _, err := source.resolve(context.Background(), source.reference); err != nil {
			t.Fatalf("%s Resolve: %v", source.name, err)
		}
		durations = append(durations, time.Since(started))
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[(calls*95)/100-1]
	t.Logf("cold fake calls = %d, p95 = %s", calls, p95)
	if p95 > time.Millisecond {
		t.Fatalf("cold fake p95 = %s, budget = 1ms", p95)
	}
}

type directSecretPerformanceSource struct {
	name      string
	reference Reference
	resolve   func(context.Context, Reference) (sourceMaterial, error)
}

func directSecretPerformanceSources(t *testing.T) []directSecretPerformanceSource {
	t.Helper()
	value := "valid"
	azureID := azsecrets.ID("https://vault.test/secrets/key/1")
	return []directSecretPerformanceSource{
		{
			name: "gcp", reference: mustReference(t, "gcp-secret-manager:projects/project/secrets/key"),
			resolve: (&gcpSecretManagerSource{open: func(context.Context) (gcpSecretRead, func() error, error) {
				return func(context.Context, string) (*secretmanagerpb.AccessSecretVersionResponse, error) {
					return &secretmanagerpb.AccessSecretVersionResponse{
						Name:    "projects/project/secrets/key/versions/1",
						Payload: &secretmanagerpb.SecretPayload{Data: []byte(value)},
					}, nil
				}, func() error { return nil }, nil
			}}).Resolve,
		},
		{
			name: "azure", reference: mustReference(t, "azure-key-vault:https://vault.test/secrets/key"),
			resolve: (&azureKeyVaultSource{open: func(string) (azureSecretRead, func() error, error) {
				return func(context.Context, string, string) (azsecrets.GetSecretResponse, error) {
					return azsecrets.GetSecretResponse{Secret: azsecrets.Secret{Value: &value, ID: &azureID}}, nil
				}, func() error { return nil }, nil
			}}).Resolve,
		},
		{
			name: "aws", reference: mustReference(t, "aws-secrets-manager:key"),
			resolve: (&awsSecretsManagerSource{open: func(context.Context) (awsSecretRead, func() error, error) {
				return func(context.Context, *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
					return &secretsmanager.GetSecretValueOutput{
						SecretString: &value, VersionId: aws.String("1"),
					}, nil
				}, func() error { return nil }, nil
			}}).Resolve,
		},
		{
			name: "vault", reference: mustReference(t, "vault:secret/key"),
			resolve: (&vaultSource{open: func() (vaultSecretRead, func() error, error) {
				return func(context.Context, string, string, int) (*vault.KVSecret, error) {
					return &vault.KVSecret{
						Data:            map[string]any{"value": value},
						VersionMetadata: &vault.KVVersionMetadata{Version: 1},
					}, nil
				}, func() error { return nil }, nil
			}}).Resolve,
		},
	}
}

func TestScalarSecretFieldSelectionRejectsDuplicatesAndNonStrings(t *testing.T) {
	for _, payload := range []string{
		`{"api-key":"one","api-key":"two"}`,
		`{"api-key":7}`,
		`[]`,
		`{"api-key":"one"} trailing`,
	} {
		_, err := scalarSecretMaterial(
			referenceBackendAWSStore, []byte(payload), "api-key", "1",
		)
		if !isSourceError(err, SourceErrorInvalid) {
			t.Fatalf("payload %q error = %v", payload, err)
		}
	}
}

func TestDirectSecretSourceErrorsDoNotExposeReferences(t *testing.T) {
	const sensitive = "customer-a-secret"
	source := &awsSecretsManagerSource{open: func(context.Context) (awsSecretRead, func() error, error) {
		return func(context.Context, *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
			return nil, &smithy.GenericAPIError{Code: "AccessDeniedException", Message: sensitive}
		}, func() error { return nil }, nil
	}}
	_, err := source.Resolve(
		context.Background(), mustReference(t, "aws-secrets-manager:"+sensitive),
	)
	if err == nil || strings.Contains(err.Error(), sensitive) {
		t.Fatalf("error exposed source details: %v", err)
	}
}

func referenceBackends(backends []ReferenceBackend) []string {
	values := make([]string, len(backends))
	for index, backend := range backends {
		values[index] = string(backend)
	}
	return values
}
