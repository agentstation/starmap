package transport

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/internal/auth"
)

type contextKey string

type contextCheckingRoundTripper struct {
	t *testing.T
}

func TestClientRejectsCrossOriginRedirectBeforeCredentialForwarding(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		targetCalls.Add(1)
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/stolen", http.StatusFound)
	}))
	defer origin.Close()

	response, err := New(auth.ResolvedAuth{}).Get(context.Background(), origin.URL+"/models")
	if response != nil {
		_ = response.Body.Close()
	}
	if err == nil {
		t.Fatal("cross-origin redirect must fail")
	}
	if targetCalls.Load() != 0 {
		t.Fatalf("redirect target received %d requests", targetCalls.Load())
	}
}

func TestClientAllowsSameOriginRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			http.Redirect(w, r, "/v1/models", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	response, err := New(auth.ResolvedAuth{}).Get(context.Background(), server.URL+"/models")
	if err != nil {
		t.Fatalf("same-origin redirect: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.Request.URL.Path != "/v1/models" {
		t.Fatalf("final path = %q", response.Request.URL.Path)
	}
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

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
}
