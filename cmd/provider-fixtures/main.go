// Command provider-fixtures manages governed provider response fixtures.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/acquisition"
	responsefixtures "github.com/agentstation/starmap/internal/providers/fixtures/responses"
	"github.com/agentstation/starmap/internal/providers/registry"
	"github.com/agentstation/starmap/pkg/catalogmeta"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

var providerPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, errors.SafeSummary(err))
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return &errors.ValidationError{Field: "command", Message: "refresh, import, or replay is required"}
	}
	switch args[0] {
	case "refresh":
		return runRefresh(ctx, args[1:])
	case "import":
		return runImport(ctx, args[1:])
	case "replay":
		return runReplay(ctx, args[1:])
	default:
		return &errors.ValidationError{Field: "command", Value: args[0], Message: "must be refresh, import, or replay"}
	}
}

func runRefresh(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("provider-fixtures refresh", flag.ContinueOnError)
	providerID := flags.String("provider", "", "provider ID registered in the embedded catalog")
	sourceID := flags.String("source", "", "logical provider source ID (required when the provider has multiple sources)")
	outputRoot := flags.String("output-root", filepath.Join("internal", "providers", "fixtures", "responses"), "governed provider observation root")
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
	selectedSource, err := selectSource(provider, *sourceID, false)
	if err != nil {
		return err
	}
	forbidden := configuredSecrets(&provider)
	fixturePath := filepath.Join(*outputRoot, *providerID, selectedSource.ID, "models_list.json")
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	result, err := responsefixtures.Refresh(ctx, responsefixtures.RefreshOptions{
		Provider: *providerID, Source: selectedSource.ID, FixturePath: fixturePath, Now: time.Now().UTC(), ForbiddenBytes: forbidden,
		Fetch: func(fetchCtx context.Context) (responsefixtures.FetchResult, error) {
			resolved, err := acquisition.NewResolver().Resolve(fetchCtx, &provider, selectedSource.ID)
			if err != nil {
				return responsefixtures.FetchResult{}, err
			}
			fetched, err := registry.FetchRaw(fetchCtx, resolved)
			if err != nil {
				return responsefixtures.FetchResult{}, err
			}
			return responsefixtures.FetchResult{Payload: fetched.Data, Revision: responseRevision(fetched.Header, fetched.Data)}, nil
		},
		Validate: func(validateCtx context.Context, payload []byte) error {
			return validatePayload(validateCtx, provider, selectedSource.ID, payload)
		},
	})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "refreshed provider=%s source=%s bytes=%d checksum=%s\n", result.Provider, result.Source, result.Bytes, result.Checksum)
	return nil
}

func validatePayload(ctx context.Context, provider catalogs.Provider, sourceID string, payload []byte) error {
	models, err := decodeFixtureModels(ctx, provider, sourceID, payload)
	if err != nil {
		return err
	}
	if len(models) == 0 {
		return &errors.ValidationError{Field: "provider_response_fixture.models", Message: "validated payload must contain at least one model"}
	}
	return nil
}

func decodeFixtureModels(ctx context.Context, provider catalogs.Provider, sourceID string, payload []byte) ([]catalogs.Model, error) {
	selected, err := selectSource(provider, sourceID, true)
	if err != nil {
		return nil, err
	}
	selected.Auth = catalogs.ProviderAuthPolicy{Mode: catalogs.ProviderAuthModeNone}
	provider.Catalog.Sources = []catalogs.ProviderSource{selected}
	resolved, err := acquisition.NewResolver().Resolve(ctx, &provider, selected.ID)
	if err != nil {
		return nil, err
	}
	models, err := registry.DecodeFixture(resolved, payload)
	if err != nil {
		return nil, err
	}
	return models, nil
}

func selectSource(provider catalogs.Provider, requested string, requireExplicit bool) (catalogs.ProviderSource, error) {
	if provider.Catalog == nil || len(provider.Catalog.Sources) == 0 {
		return catalogs.ProviderSource{}, &errors.ValidationError{Field: "provider.catalog.sources", Message: "must contain at least one logical source"}
	}
	requested = strings.TrimSpace(requested)
	if requested == "" {
		if requireExplicit || len(provider.Catalog.Sources) != 1 {
			return catalogs.ProviderSource{}, &errors.ValidationError{Field: "source", Message: "is required for exact logical-source fixture identity"}
		}
		return provider.Catalog.Sources[0], nil
	}
	for _, source := range provider.Catalog.Sources {
		if source.ID == requested {
			return source, nil
		}
	}
	return catalogs.ProviderSource{}, &errors.NotFoundError{Resource: "provider source", ID: string(provider.ID) + "/" + requested}
}

func configuredSecrets(provider *catalogs.Provider) [][]byte {
	values := make([][]byte, 0, len(provider.Credentials))
	for _, credential := range provider.Credentials {
		for _, name := range credential.Env {
			value, found := os.LookupEnv(name)
			if found && sensitiveEnvironmentName(name) && strings.TrimSpace(value) != "" {
				values = append(values, []byte(value))
			}
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

func responseRevision(header http.Header, payload []byte) catalogmeta.ObservationRevision {
	if header != nil {
		if value := strings.TrimSpace(header.Get("ETag")); value != "" {
			return catalogmeta.ObservationRevision{Kind: catalogmeta.ObservationRevisionKindETag, Value: value}
		}
		if value := strings.TrimSpace(header.Get("Last-Modified")); value != "" {
			return catalogmeta.ObservationRevision{Kind: catalogmeta.ObservationRevisionKindLastModified, Value: value}
		}
	}
	return catalogmeta.ObservationRevision{Kind: catalogmeta.ObservationRevisionKindContentDigest, Value: responsefixtures.Checksum(payload)}
}
