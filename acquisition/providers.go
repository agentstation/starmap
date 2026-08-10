package acquisition

import (
	"context"

	"github.com/agentstation/starmap/internal/auth"
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
	composed := make([]sources.ProviderOption, 0, 2+len(opts))
	composed = append(
		composed,
		sources.WithProviderClientFactory(defaultProviderClientFactory),
		sources.WithProviderRawFetcher(defaultRawFetcher),
		sources.WithProviderCredentialResolver(auth.NewResolver()),
	)
	composed = append(composed, opts...)
	return sources.NewProviderFetcher(providers, composed...)
}

func defaultRawFetcher(
	ctx context.Context,
	provider *catalogs.Provider,
	material sources.ProviderCredentialMaterial,
	endpoint string,
) (*sources.RawFetchResult, error) {
	result, err := clients.FetchRaw(ctx, provider, material, endpoint)
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
