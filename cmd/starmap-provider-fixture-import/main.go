// Command starmap-provider-fixture-import converts one integrity-bound raw
// provider fixture into canonical provider-model YAML without network access.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentstation/starmap/internal/providers/clients"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/constants"
	"github.com/agentstation/starmap/pkg/errors"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("starmap-provider-fixture-import", flag.ContinueOnError)
	providerID := flags.String("provider", "", "provider ID registered in the embedded catalog")
	fixturePath := flags.String("fixture", "", "raw provider response fixture")
	outputRoot := flags.String("output", "", "catalog root receiving providers/<id>/models")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *providerID == "" || *fixturePath == "" || *outputRoot == "" {
		return &errors.ValidationError{Field: "arguments", Message: "provider, fixture, and output are required"}
	}
	fixture, err := os.ReadFile(*fixturePath) //nolint:gosec // Explicit operator-controlled fixture path.
	if err != nil {
		return errors.WrapIO("read", *fixturePath, err)
	}

	builder, err := catalogs.NewEmbedded()
	if err != nil {
		return errors.WrapResource("load", "embedded catalog", "", err)
	}
	provider, err := builder.Provider(catalogs.ProviderID(*providerID))
	if err != nil {
		return err
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(fixture)
	}))
	defer server.Close()
	provider.APIKey = nil
	provider.EnvVars = nil
	provider.EnvVarValues = nil
	provider.Catalog.Endpoint.URL = server.URL
	provider.Catalog.Endpoint.BaseURLEnvVar = ""
	provider.Catalog.Endpoint.Path = ""
	provider.Catalog.Endpoint.AuthRequired = false
	if provider.Catalog.Offering != nil && provider.Catalog.Offering.Endpoint.BaseURL == "" {
		provider.Catalog.Offering.Endpoint.BaseURL = server.URL
	}

	client, err := clients.NewProvider(&provider)
	if err != nil {
		return errors.WrapResource("create", "provider fixture client", *providerID, err)
	}
	models, err := client.ListModels(ctx)
	if err != nil {
		return errors.WrapResource("decode", "provider fixture", *providerID, err)
	}
	if len(models) == 0 {
		return &errors.ValidationError{Field: "fixture.models", Message: "must contain at least one accepted public model"}
	}
	for _, model := range models {
		relative, err := safeModelPath(*providerID, model.ID)
		if err != nil {
			return err
		}
		payload, err := model.EncodeYAML()
		if err != nil {
			return errors.WrapResource("encode", "provider model", model.ID, err)
		}
		if err := writeAtomic(filepath.Join(*outputRoot, relative), []byte(payload)); err != nil {
			return err
		}
	}
	_, _ = fmt.Fprintf(os.Stdout, "imported %d models for %s\n", len(models), *providerID)
	return nil
}

func safeModelPath(providerID, modelID string) (string, error) {
	if strings.TrimSpace(providerID) == "" || strings.TrimSpace(modelID) == "" || filepath.IsAbs(modelID) || strings.Contains(modelID, `\`) {
		return "", &errors.ValidationError{Field: "model.id", Value: modelID, Message: "is not a safe catalog path"}
	}
	clean := filepath.Clean(filepath.FromSlash(modelID))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", &errors.ValidationError{Field: "model.id", Value: modelID, Message: "must remain within the provider model directory"}
	}
	return filepath.Join("providers", providerID, "models", clean+".yaml"), nil
}

func writeAtomic(path string, payload []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, constants.DirPermissions); err != nil {
		return errors.WrapIO("create", directory, err)
	}
	temporary, err := os.CreateTemp(directory, ".provider-model-*.tmp")
	if err != nil {
		return errors.WrapIO("create", path, err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(constants.FilePermissions); err != nil {
		_ = temporary.Close()
		return errors.WrapIO("chmod", temporaryPath, err)
	}
	if _, err := temporary.Write(payload); err != nil {
		_ = temporary.Close()
		return errors.WrapIO("write", temporaryPath, err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return errors.WrapIO("sync", temporaryPath, err)
	}
	if err := temporary.Close(); err != nil {
		return errors.WrapIO("close", temporaryPath, err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return errors.WrapIO("rename", path, err)
	}
	return nil
}
