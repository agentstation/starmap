package productpaths_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap/internal/privatefiles"
	"github.com/agentstation/starmap/pkg/productpaths"
	"github.com/agentstation/starmap/pkg/productpaths/policy"
)

func TestConfigurationInputUsesCheckedBoundedReads(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "starport")
	files, err := privatefiles.NewDirectory(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := files.WriteFile("config.env", []byte("CATALOG=embedded\n"), ".input-"); err != nil {
		t.Fatal(err)
	}
	input := productpaths.ConfigurationInput{Path: filepath.Join(directory, "config.env"), MaxBytes: 64}
	for _, reader := range []func() ([]byte, error){
		func() ([]byte, error) { return productpaths.ReadConfiguration(t.Context(), input) },
		func() ([]byte, error) { return productpaths.ReadDotenv(t.Context(), input.Path, input.MaxBytes) },
	} {
		got, err := reader()
		if err != nil || string(got) != "CATALOG=embedded\n" {
			t.Fatalf("private configuration read failed: %q, %v", got, err)
		}
	}
	input.MaxBytes = 3
	if _, err := productpaths.ReadConfiguration(t.Context(), input); err == nil {
		t.Fatal("configuration exceeded the byte limit")
	}
	input.MaxBytes = 64
	input.AccessPolicy = policy.ServiceManaged
	if _, err := productpaths.ReadConfiguration(t.Context(), input); err == nil {
		t.Fatal("service-managed configuration accepted an implicit path")
	}
	input.Explicit = true
	if _, err := productpaths.ReadConfiguration(t.Context(), input); err != nil {
		t.Fatal(err)
	}
}

func TestConfigurationInputRejectsCancellationBeforeAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	input := productpaths.ConfigurationInput{Path: filepath.Join(t.TempDir(), "missing.env"), MaxBytes: 64}
	if _, err := productpaths.ReadConfiguration(ctx, input); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read error: %v", err)
	}
	if _, err := productpaths.ReadDotenv(ctx, input.Path, input.MaxBytes); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled dotenv read error: %v", err)
	}
}
