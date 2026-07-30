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

	"github.com/agentstation/starmap/pkg/catalogs"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

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
		auth: &NoAuth{},
	}

	ctx := context.WithValue(context.Background(), contextKey("request-id"), "expected")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/models", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req, nil)
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

	client := New(nil)
	response, err := client.Get(context.Background(), origin.URL, nil)
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
	t.Setenv("QUERY_AUTH_KEY", secret)
	provider := &catalogs.Provider{
		ID: "query-auth",
		APIKey: &catalogs.ProviderAPIKey{
			Name:       "QUERY_AUTH_KEY",
			QueryParam: "key",
		},
	}
	client := &Client{
		http: &http.Client{Transport: secretErrorRoundTripper{}},
		auth: newAuthenticator(provider),
	}
	request, err := http.NewRequest(
		http.MethodGet,
		"https://provider.example/models",
		nil,
	)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	_, err = client.Do(request, provider)
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
