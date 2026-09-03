// Package channel provides one synthetic catalog channel for application
// tests. The channel runs on a local test server, so an application test
// exercises the real pull path and reaches no public network.
package channel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

const (
	// ConfiguredEnvironment names the credential of the eligible provider. A
	// test sets it to a placeholder that reaches only the local test server.
	ConfiguredEnvironment = "STARMAP_TEST_CONFIGURED_API_KEY"

	// AbsentEnvironment names a credential that no test sets. The provider that
	// needs it must send no request.
	AbsentEnvironment = "STARMAP_TEST_ABSENT_API_KEY"

	// ConfiguredProvider is the provider that holds a credential.
	ConfiguredProvider catalogs.ProviderID = "provider-configured"

	// UnconfiguredProvider is the provider that holds none.
	UnconfiguredProvider catalogs.ProviderID = "provider-unconfigured"

	// ObservedModel is the model slug that the channel and the provider share.
	ObservedModel = "model-observed"

	// AuthorID owns the canonical authored model of the channel.
	AuthorID catalogs.AuthorID = "test-author"

	// GenerationID names the one generation that the channel publishes.
	GenerationID = "synthetic-1"
)

// Upstream is the local test server that publishes one synthetic channel and
// answers one provider model request. It is also the source that reads that
// channel, so a test injects it with starmap.WithSource.
type Upstream struct {
	server       *httptest.Server
	payload      []byte
	channelReads atomic.Int64
	modelReads   atomic.Int64
	served       atomic.Bool
}

// Start builds the synthetic catalog and starts the local test server.
func Start() (*Upstream, error) {
	upstream := &Upstream{}
	mux := http.NewServeMux()
	mux.HandleFunc("/channel", upstream.handleChannel)
	mux.HandleFunc("/v1/models", upstream.handleModels)
	upstream.server = httptest.NewServer(mux)

	payload, err := channelPayload(upstream.server.URL + "/v1/models")
	if err != nil {
		upstream.server.Close()
		return nil, err
	}
	upstream.payload = payload
	return upstream, nil
}

// Close stops the local test server. A test calls it to simulate an outage.
func (u *Upstream) Close() { u.server.Close() }

// URL returns the base address of the local test server.
func (u *Upstream) URL() string { return u.server.URL }

// ChannelReads counts the channel reads that the runtime made.
func (u *Upstream) ChannelReads() int64 { return u.channelReads.Load() }

// ModelReads counts the provider model requests that reached the server.
func (u *Upstream) ModelReads() int64 { return u.modelReads.Load() }

// Identity names the source without an address or a credential.
func (u *Upstream) Identity() string { return "synthetic-channel" }

// Read checks the channel and returns the generation on the first change. A
// later read reports no change, so the runtime keeps its durable generation.
func (u *Upstream) Read(ctx context.Context) (starmap.SourceRead, error) {
	request, err := http.NewRequestWithContext(
		ctx, http.MethodGet, u.server.URL+"/channel", nil)
	if err != nil {
		return starmap.SourceRead{}, errors.WrapIO("build", "synthetic channel request", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return starmap.SourceRead{}, errors.WrapIO("read", "synthetic channel", err)
	}
	defer func() { _ = response.Body.Close() }()
	var channel struct {
		Sequence int `json:"sequence"`
	}
	if err := json.NewDecoder(response.Body).Decode(&channel); err != nil {
		return starmap.SourceRead{}, errors.WrapIO("decode", "synthetic channel", err)
	}

	published := time.Unix(1, 0).UTC()
	read := starmap.SourceRead{Health: starmap.HealthOK, ChannelUpdatedAt: published}
	if !u.served.CompareAndSwap(false, true) {
		return read, nil
	}
	read.Changed = true
	read.PublishedAt = published
	read.Generation = catalogs.Generation{
		Manifest: catalogs.GenerationManifest{
			GenerationID: GenerationID,
			GeneratedAt:  published,
			Payload:      catalogs.DescribeCatalogPayload(u.payload),
		},
		Payload: u.payload,
	}
	return read, nil
}

// handleChannel answers the channel check.
func (u *Upstream) handleChannel(w http.ResponseWriter, _ *http.Request) {
	u.channelReads.Add(1)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"sequence":1}`))
}

// handleModels answers one provider model request. It rejects a request that
// carries no credential, so the counters expose every leaked request.
func (u *Upstream) handleModels(w http.ResponseWriter, r *http.Request) {
	u.modelReads.Add(1)
	if r.Header.Get("Authorization") == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(
		[]byte(`{"object":"list","data":[{"id":"` + ObservedModel + `","object":"model"}]}`))
}

// channelPayload encodes the synthetic catalog that the channel publishes. One
// provider holds a credential and one holds none.
func channelPayload(endpoint string) ([]byte, error) {
	author := catalogs.Author{ID: AuthorID, Name: "Test Author"}

	builder := catalogs.NewEmpty()
	if err := builder.SetAuthor(author); err != nil {
		return nil, errors.WrapResource("set", "synthetic author", string(AuthorID), err)
	}
	if err := builder.SetAuthorModel(AuthorID, catalogs.Model{
		ID: ObservedModel, Name: "Model Observed", Authors: []catalogs.Author{author},
	}); err != nil {
		return nil, errors.WrapResource("set", "synthetic authored model", ObservedModel, err)
	}
	for _, provider := range []catalogs.Provider{
		Provider(ConfiguredProvider, endpoint, ConfiguredEnvironment),
		Provider(UnconfiguredProvider, endpoint, AbsentEnvironment),
	} {
		if err := builder.SetProvider(provider); err != nil {
			return nil, errors.WrapResource("set", "synthetic provider", string(provider.ID), err)
		}
		if err := builder.SetProviderModel(provider.ID, catalogs.Model{
			ID:       ObservedModel,
			Name:     "Model Observed",
			ModelRef: catalogs.AuthoredModelID(AuthorID, ObservedModel),
		}); err != nil {
			return nil, errors.WrapResource(
				"set", "synthetic provider model", string(provider.ID), err)
		}
	}
	catalog, err := builder.Build()
	if err != nil {
		return nil, errors.WrapResource("publish", "synthetic catalog", GenerationID, err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		return nil, errors.WrapResource("encode", "synthetic catalog", GenerationID, err)
	}
	return payload, nil
}

// Provider returns one OpenAI-compatible provider that needs the named
// environment credential.
func Provider(
	id catalogs.ProviderID,
	endpoint, environment string,
) catalogs.Provider {
	return catalogs.Provider{
		ID:   id,
		Name: string(id),
		Catalog: &catalogs.ProviderCatalog{
			Endpoint: catalogs.ProviderEndpoint{
				Type: catalogs.EndpointTypeOpenAI,
				URL:  endpoint,
				ProtocolOptions: catalogs.ProviderCatalogProtocolOptions{
					OpenAI: &catalogs.ProviderOpenAICatalogProtocolOptions{
						TokenPriceUnit: catalogs.ProviderTokenPriceUnitPerMillion,
					},
				},
			},
		},
		Credentials: &catalogs.ProviderCredentials{
			Fields: []catalogs.ProviderCredentialField{{
				ID: "api-key", Kind: catalogs.ProviderCredentialFieldSecret,
				Required: true, Environment: []string{environment},
			}},
			Profiles: []catalogs.ProviderCredentialProfile{{
				ID: "api-key", Primitive: catalogs.ProviderAuthenticationAPIKey,
				Fields: []catalogs.ProviderCredentialFieldID{"api-key"},
				Placements: []catalogs.ProviderCredentialPlacement{{
					Field: "api-key", Kind: catalogs.ProviderCredentialPlacementHeader,
					Name: "Authorization", Scheme: catalogs.ProviderCredentialSchemeBearer,
				}},
			}},
			CatalogAcquisition: catalogs.ProviderCredentialPlane{
				Required:     true,
				Alternatives: []catalogs.ProviderCredentialProfileID{"api-key"},
			},
			Inference: catalogs.ProviderCredentialPlane{
				Required:     true,
				Alternatives: []catalogs.ProviderCredentialProfileID{"api-key"},
			},
		},
	}
}
