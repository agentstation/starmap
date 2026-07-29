package acquisition

import (
	"context"

	"github.com/agentstation/starmap/internal/providers/clients"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
)

// NewProviderFetcher returns the repository's opt-in concrete provider
// composition. Importing starmap alone never imports provider SDKs.
func NewProviderFetcher(
	providers catalogs.ProvidersReader,
	opts ...sources.ProviderOption,
) *sources.ProviderFetcher {
	composed := []sources.ProviderOption{
		sources.WithProviderClientFactory(defaultProviderClientFactory),
		sources.WithProviderRawFetcher(defaultRawFetcher),
	}
	composed = append(composed, opts...)
	return sources.NewProviderFetcher(providers, composed...)
}

func defaultRawFetcher(
	ctx context.Context,
	provider *catalogs.Provider,
	endpoint string,
) (*sources.RawFetchResult, error) {
	result, err := clients.FetchRaw(ctx, provider, endpoint)
	if err != nil {
		return nil, err
	}
	return &sources.RawFetchResult{
		Data:       result.Data,
		Response:   result.Response,
		Latency:    result.Latency,
		RequestURL: result.RequestURL,
	}, nil
}
