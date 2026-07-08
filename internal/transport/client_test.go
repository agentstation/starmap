package transport

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
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
