package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/constants"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/remote"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

// testIdleBound is the inactivity bound of the stalled-body test. It is short,
// so the test finishes quickly.
const testIdleBound = 100 * time.Millisecond

// testMaxTransferBound keeps the stalled-body test at its inactivity bound
// rather than its per-transfer maximum.
const testMaxTransferBound = 30 * time.Second

func TestProviderTransferPolicyBoundsEveryBody(t *testing.T) {
	policy := providerTransferPolicy()
	if policy.IdleTimeout != remote.DefaultTransferIdleTimeout {
		t.Fatalf("idle timeout = %s, want %s",
			policy.IdleTimeout, remote.DefaultTransferIdleTimeout)
	}
	if policy.MaxDuration != remote.DefaultTransferMaxDuration {
		t.Fatalf("max duration = %s, want %s",
			policy.MaxDuration, remote.DefaultTransferMaxDuration)
	}
	if policy.MaxCompressedBytes != constants.MaxSourcePayloadBytes+1 {
		t.Fatalf("max compressed bytes = %d, want %d",
			policy.MaxCompressedBytes, constants.MaxSourcePayloadBytes+1)
	}
}

func TestProviderBodyStopsAtTheIdleBound(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}
		// Hold the body open and send nothing.
		<-release
	}))
	t.Cleanup(func() {
		close(release)
		server.Close()
	})

	policy := providerTransferPolicy()
	policy.IdleTimeout = testIdleBound
	policy.MaxDuration = testMaxTransferBound
	client := newClient(policy)

	response, err := client.Get(
		context.Background(), server.URL, nil, sources.ProviderCredentialMaterial{},
	)
	if err == nil {
		_ = response.Body.Close()
		t.Fatal("Get read a stalled provider body without a bound")
	}
	var timeout *pkgerrors.TimeoutError
	if !errors.As(err, &timeout) {
		t.Fatalf("error = %T, want *errors.TimeoutError", err)
	}
	if timeout.Duration != testIdleBound.String() {
		t.Fatalf("timeout duration = %q, want %q", timeout.Duration, testIdleBound)
	}
}

func TestNewSetsNoClientWideTimeout(t *testing.T) {
	client := New()
	if client.http.Timeout != 0 {
		t.Fatalf("client timeout = %s, want no client-wide timeout", client.http.Timeout)
	}
	transport, ok := client.http.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("client transport = %T, want *http.Transport", client.http.Transport)
	}
	if transport.ResponseHeaderTimeout != remote.DefaultResponseHeaderTimeout {
		t.Fatalf("response header timeout = %s, want %s",
			transport.ResponseHeaderTimeout, remote.DefaultResponseHeaderTimeout)
	}
	if transport.TLSHandshakeTimeout != remote.DefaultTLSHandshakeTimeout {
		t.Fatalf("TLS handshake timeout = %s, want %s",
			transport.TLSHandshakeTimeout, remote.DefaultTLSHandshakeTimeout)
	}
	if client.http.CheckRedirect == nil {
		t.Fatal("the client follows redirects while it carries provider credentials")
	}
}

type contextKey string

type contextCheckingRoundTripper struct {
	t *testing.T
}

func (rt contextCheckingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.t.Helper()
	if req.Context().Value(contextKey("request-id")) != "expected" {
		rt.t.Fatal("transport request did not preserve original request context")
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("{}")),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func TestDoPreservesRequestContext(t *testing.T) {
	client := &Client{
		http: &http.Client{Transport: contextCheckingRoundTripper{t: t}},
	}

	ctx := context.WithValue(context.Background(), contextKey("request-id"), "expected")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/models", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req, nil, sources.ProviderCredentialMaterial{})
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
}

func TestProviderTransportNeverFollowsRedirects(t *testing.T) {
	var redirected atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(
		http.ResponseWriter,
		*http.Request,
	) {
		redirected.Add(1)
	}))
	defer target.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		http.Redirect(writer, &http.Request{}, target.URL, http.StatusFound)
	}))
	defer origin.Close()

	client := New()
	response, err := client.Get(
		context.Background(), origin.URL, nil, sources.ProviderCredentialMaterial{},
	)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want un-followed redirect", response.StatusCode)
	}
	if redirected.Load() != 0 {
		t.Fatal("provider transport followed redirect target")
	}
}

type secretErrorRoundTripper struct{}

func (secretErrorRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return nil, errors.New("dial failed for " + request.URL.String())
}

func TestQueryCredentialIsAbsentFromTransportErrorGraph(t *testing.T) {
	const secret = "do-not-leak"
	provider := &catalogs.Provider{
		ID: "query-auth",
	}
	profile := catalogs.ProviderCredentialProfile{
		ID:        "api-key",
		Primitive: catalogs.ProviderAuthenticationAPIKey,
		Fields:    []catalogs.ProviderCredentialFieldID{"api-key"},
		Placements: []catalogs.ProviderCredentialPlacement{{
			Field: "api-key", Kind: catalogs.ProviderCredentialPlacementQuery,
			Name: "key", Scheme: catalogs.ProviderCredentialSchemeDirect,
		}},
	}
	material := sources.NewProviderCredentialMaterial(
		profile,
		map[catalogs.ProviderCredentialFieldID]string{"api-key": secret},
		sources.ProviderCredentialMetadata{Version: "test"},
	)
	client := &Client{
		http: &http.Client{Transport: secretErrorRoundTripper{}},
	}
	request, err := http.NewRequest(
		http.MethodGet,
		"https://provider.example/models",
		nil,
	)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	_, err = client.Do(request, provider, material)
	if err == nil {
		t.Fatal("Do returned nil error")
	}
	var apiErr *pkgerrors.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want *errors.APIError", err)
	}
	for current := err; current != nil; current = errors.Unwrap(current) {
		if strings.Contains(current.Error(), secret) {
			t.Fatalf("credential exposed in error graph: %v", current)
		}
	}
}
