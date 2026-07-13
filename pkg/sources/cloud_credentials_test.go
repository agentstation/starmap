package sources

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestCloudCredentialChainFallbackAndSecretIsolation(t *testing.T) {
	chain, err := NewCloudCredentialChain("aws",
		CloudCredentialResolver[string]{Name: "environment", Resolve: func(context.Context) (string, bool, error) { return "", false, nil }},
		CloudCredentialResolver[string]{Name: "workload-identity", Resolve: func(context.Context) (string, bool, error) { return "secret-token", true, nil }},
	)
	if err != nil {
		t.Fatalf("NewCloudCredentialChain: %v", err)
	}
	credential, err := chain.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if credential.Source != "workload-identity" || credential.Value() != "secret-token" {
		t.Fatalf("credential = %#v", credential)
	}
	encoded, err := json.Marshal(credential)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(encoded), "secret-token") {
		t.Fatalf("serialized credential leaked secret: %s", encoded)
	}
}

func TestCloudCredentialChainCancellationAndFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	chain, err := NewCloudCredentialChain("azure", CloudCredentialResolver[string]{
		Name: "managed-identity", Resolve: func(context.Context) (string, bool, error) { return "", false, errors.New("must not run") },
	})
	if err != nil {
		t.Fatalf("NewCloudCredentialChain: %v", err)
	}
	if _, err := chain.Resolve(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Resolve canceled error = %v", err)
	}

	missing, err := NewCloudCredentialChain("azure", CloudCredentialResolver[string]{
		Name: "environment", Resolve: func(context.Context) (string, bool, error) { return "", false, nil },
	})
	if err != nil {
		t.Fatalf("NewCloudCredentialChain missing: %v", err)
	}
	if _, err := missing.Resolve(context.Background()); err == nil {
		t.Fatal("Resolve missing returned nil error")
	}
}
