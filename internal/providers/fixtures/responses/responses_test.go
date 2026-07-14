package responses

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/constants"
)

func TestRefreshPromotesValidatedPayloadAndMetadata(t *testing.T) {
	fixture := filepath.Join(t.TempDir(), "responses", "provider", "models", "models_list.json")
	now := time.Date(2026, time.July, 12, 20, 0, 0, 0, time.UTC)
	result, err := Refresh(context.Background(), RefreshOptions{
		Provider: "provider", Source: "models", FixturePath: fixture, Now: now,
		Fetch: func(context.Context) (FetchResult, error) {
			return FetchResult{Payload: []byte(`{"data":[{"id":"model"}]}`)}, nil
		},
		Validate: func(_ context.Context, payload []byte) error {
			if len(payload) == 0 {
				t.Fatal("validator received empty payload")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if result.Provider != "provider" || result.Bytes == 0 || result.Checksum == "" {
		t.Fatalf("result = %#v", result)
	}
	if err := Verify(fixture, now); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestRefreshRejectsFailureNoOpAndSecretWithoutMutation(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "responses", "provider", "models")
	if err := os.MkdirAll(directory, constants.DirPermissions); err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(directory, "models_list.json")
	original := []byte(`{"data":[{"id":"old"}]}`)
	if err := os.WriteFile(fixture, original, constants.SecureFilePermissions); err != nil {
		t.Fatal(err)
	}
	fetchFailure := stderrors.New("fetch failed")
	tests := []struct {
		name      string
		payload   []byte
		fetchErr  error
		validate  error
		forbidden [][]byte
	}{
		{name: "fetch failure", fetchErr: fetchFailure},
		{name: "validation failure", payload: []byte(`{"data":[]}`), validate: stderrors.New("invalid fixture")},
		{name: "no-op", payload: original},
		{name: "secret", payload: []byte(`{"token":"fixture-secret"}`), forbidden: [][]byte{[]byte("fixture-secret")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Refresh(context.Background(), RefreshOptions{
				Provider: "provider", Source: "models", FixturePath: fixture, Now: time.Now().UTC(), ForbiddenBytes: test.forbidden,
				Fetch:    func(context.Context) (FetchResult, error) { return FetchResult{Payload: test.payload}, test.fetchErr },
				Validate: func(context.Context, []byte) error { return test.validate },
			})
			if err == nil {
				t.Fatal("Refresh unexpectedly succeeded")
			}
			got, readErr := os.ReadFile(fixture)
			if readErr != nil || string(got) != string(original) {
				t.Fatalf("fixture changed after failure: %q/%v", got, readErr)
			}
		})
	}
}
