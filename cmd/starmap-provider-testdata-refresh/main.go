// Command starmap-provider-testdata-refresh refreshes one raw provider fixture.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/providerfixture"
	"github.com/agentstation/starmap/internal/providers/clients"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

var providerPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("starmap-provider-testdata-refresh", flag.ContinueOnError)
	providerID := flags.String("provider", "", "provider ID registered in the embedded catalog")
	outputRoot := flags.String("output-root", filepath.Join("internal", "providers"), "provider package root")
	timeout := flags.Duration("timeout", 30*time.Second, "provider request timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !providerPattern.MatchString(*providerID) {
		return &errors.ValidationError{Field: "provider", Value: *providerID, Message: "must use lowercase letters, digits, or hyphens"}
	}
	if *timeout <= 0 {
		return &errors.ValidationError{Field: "timeout", Value: *timeout, Message: "must be positive"}
	}
	builder, err := catalogs.NewEmbedded()
	if err != nil {
		return errors.WrapResource("load", "embedded catalog", "", err)
	}
	provider, err := builder.Provider(catalogs.ProviderID(*providerID))
	if err != nil {
		return err
	}
	provider.LoadAPIKey()
	provider.LoadEnvVars()
	forbidden := configuredSecrets(&provider)
	fixturePath := filepath.Join(*outputRoot, *providerID, "testdata", "models_list.json")
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	result, err := providerfixture.Refresh(ctx, providerfixture.RefreshOptions{
		Provider: *providerID, FixturePath: fixturePath, Now: time.Now().UTC(), ForbiddenBytes: forbidden,
		Fetch: func(fetchCtx context.Context) (providerfixture.FetchResult, error) {
			fetched, err := clients.FetchRaw(fetchCtx, &provider, provider.CatalogEndpointURL())
			if err != nil {
				return providerfixture.FetchResult{}, err
			}
			return providerfixture.FetchResult{Payload: fetched.Data, Revision: responseRevision(fetched.Response, fetched.Data)}, nil
		},
		Validate: func(validateCtx context.Context, payload []byte) error {
			return validatePayload(validateCtx, provider, payload)
		},
	})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "refreshed provider=%s bytes=%d checksum=%s\n", result.Provider, result.Bytes, result.Checksum)
	return nil
}

func validatePayload(ctx context.Context, provider catalogs.Provider, payload []byte) error {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(payload)
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
		return err
	}
	models, err := client.ListModels(ctx)
	if err != nil {
		return err
	}
	if len(models) == 0 {
		return &errors.ValidationError{Field: "provider_fixture.models", Message: "validated payload must contain at least one model"}
	}
	return nil
}

func configuredSecrets(provider *catalogs.Provider) [][]byte {
	values := make([][]byte, 0, 1+len(provider.EnvVarValues))
	if key, err := provider.APIKeyValue(); err == nil && strings.TrimSpace(key) != "" {
		values = append(values, []byte(key))
	}
	for name, value := range provider.EnvVarValues {
		if sensitiveEnvironmentName(name) && strings.TrimSpace(value) != "" {
			values = append(values, []byte(value))
		}
	}
	return values
}

func sensitiveEnvironmentName(name string) bool {
	name = strings.ToUpper(name)
	for _, marker := range []string{"KEY", "TOKEN", "SECRET", "PASSWORD", "CREDENTIAL"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func responseRevision(response *http.Response, payload []byte) catalogmeta.ObservationRevision {
	if response != nil {
		if value := strings.TrimSpace(response.Header.Get("ETag")); value != "" {
			return catalogmeta.ObservationRevision{Kind: catalogmeta.ObservationRevisionKindETag, Value: value}
		}
		if value := strings.TrimSpace(response.Header.Get("Last-Modified")); value != "" {
			return catalogmeta.ObservationRevision{Kind: catalogmeta.ObservationRevisionKindLastModified, Value: value}
		}
	}
	return catalogmeta.ObservationRevision{Kind: catalogmeta.ObservationRevisionKindContentDigest, Value: providerfixture.Checksum(payload)}
}
