package remote

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func remotePermissionFixture() catalogs.CatalogPermissionEnvelope {
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	return catalogs.CatalogPermissionEnvelope{
		Version: catalogs.CatalogPermissionEnvelopeVersion,
		Head: catalogs.CatalogAuthorityHead{
			AuthorityID: "internal-authority", PolicyID: "production", Sequence: 10,
			GenerationID: "future-catalog", PayloadChecksum: "sha256:" + strings.Repeat("a", 64),
			RequiredPermissionRevision: "sha256:" + strings.Repeat("b", 64),
			PermissionSchemaVersion:    catalogs.CatalogPermissionSchemaVersion + 1,
		},
		IssuedAt: at, ValidUntil: at.Add(time.Minute),
	}
}

func TestRemotePermissionIndependentOfCatalogCompatibility(t *testing.T) {
	envelope := remotePermissionFixture()
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requests.Add(1)
		if r.URL.Path != "/api/v1"+PermissionEnvelopePath || r.Method != http.MethodGet || r.Header.Get("Accept") != PermissionEnvelopeMediaType {
			t.Errorf("unexpected permission request: %s %s, accept=%q", r.Method, r.URL.Path, r.Header.Get("Accept"))
			http.Error(w, "catalog format is unsupported", http.StatusBadRequest)
			return
		}
		if r.Header.Get("If-None-Match") != "" {
			t.Error("permission renewal used a conditional request")
		}
		renewed := envelope
		renewed.IssuedAt = renewed.IssuedAt.Add(time.Duration(n-1) * time.Minute)
		renewed.ValidUntil = renewed.ValidUntil.Add(time.Duration(n-1) * time.Minute)
		w.Header().Set("Content-Type", PermissionEnvelopeMediaType)
		_ = json.NewEncoder(w).Encode(renewed)
	}))
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
	defer transport.CloseIdleConnections()
	// An old catalog reader can still learn a newer mandatory permission revision.
	client, err := NewClient(server.URL+"/api/v1", &http.Client{Transport: transport}, 1)
	if err != nil {
		t.Fatal(err)
	}
	first, err := client.FetchPermissionEnvelope(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.Head != envelope.Head || first.Head.SupportsPermissions() {
		t.Fatal("required revision was lost or unsupported permissions became supported")
	}
	renewed, err := client.FetchPermissionEnvelope(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := first.ValidateSuccessor(renewed); err != nil {
		t.Fatal(err)
	}
	if !renewed.ValidUntil.After(first.ValidUntil) || renewed.Head != first.Head || requests.Load() != 2 {
		t.Fatal("receipt renewal fetched or changed the catalog")
	}
}

func TestRemotePermissionRejectsInvalidResponses(t *testing.T) {
	data, err := json.Marshal(remotePermissionFixture())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		code        int
		media, body string
		declared    int64
		chunked     bool
	}{
		{name: "unauthorized", code: http.StatusUnauthorized, media: PermissionEnvelopeMediaType, body: string(data)},
		{name: "not found", code: http.StatusNotFound, media: PermissionEnvelopeMediaType, body: string(data)},
		{name: "not modified", code: http.StatusNotModified, media: PermissionEnvelopeMediaType},
		{name: "wrong content type", code: http.StatusOK, media: ManifestMediaType, body: string(data)},
		{name: "malformed", code: http.StatusOK, media: PermissionEnvelopeMediaType, body: "{}"},
		{name: "trailing document", code: http.StatusOK, media: PermissionEnvelopeMediaType, body: string(data) + " {}"},
		{name: "declared oversized", code: http.StatusOK, media: PermissionEnvelopeMediaType, declared: catalogs.MaxCatalogPermissionEnvelopeBytes + 1},
		{name: "chunked oversized", code: http.StatusOK, media: PermissionEnvelopeMediaType, body: strings.Repeat(" ", catalogs.MaxCatalogPermissionEnvelopeBytes+1), chunked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.media)
				if tc.declared > 0 {
					w.Header().Set("Content-Length", fmt.Sprint(tc.declared))
				}
				w.WriteHeader(tc.code)
				if tc.chunked {
					w.(http.Flusher).Flush()
				}
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			client, err := NewClient(server.URL, server.Client(), catalogs.CurrentCatalogSchemaVersion)
			if err != nil {
				t.Fatal(err)
			}
			got, err := client.FetchPermissionEnvelope(context.Background())
			if err == nil || got.Version != 0 {
				t.Fatalf("invalid response returned envelope=%+v, error=%v", got, err)
			}
		})
	}
}

func TestRemotePermissionRejectsUnverifiedTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", PermissionEnvelopeMediaType)
		_ = json.NewEncoder(w).Encode(remotePermissionFixture())
	}))
	defer server.Close()
	transport := &http.Transport{TLSClientConfig: &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // Negative publisher-trust fixture.
		MinVersion:         tls.VersionTLS12,
	}}
	defer transport.CloseIdleConnections()
	client, err := NewClient(server.URL, &http.Client{Transport: transport}, catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	// A successful handshake without certificate verification does not prove the publisher.
	if _, err := client.FetchPermissionEnvelope(context.Background()); err == nil || !strings.Contains(err.Error(), "publisher") {
		t.Fatalf("unverified TLS error=%v", err)
	}
}

func TestRemotePermissionRejectsRedirectAndCancellation(t *testing.T) {
	var escaped atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { escaped.Add(1) }))
	defer other.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL, http.StatusFound) }))
	defer origin.Close()
	client, err := NewClient(origin.URL, origin.Client(), catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.FetchPermissionEnvelope(context.Background()); err == nil || escaped.Load() != 0 {
		t.Fatalf("redirect error=%v, escaped=%d", err, escaped.Load())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.FetchPermissionEnvelope(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error=%v", err)
	}
}

func TestRemotePermissionHonorsRefusalBoundary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client, err := NewClient(server.URL, server.Client(), catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.FetchPermissionEnvelope(context.Background())
	var refusal *RefusalError
	if !errors.As(err, &refusal) || refusal.StatusCode != http.StatusServiceUnavailable || refusal.NotBefore.IsZero() {
		t.Fatalf("refusal=%v", err)
	}
}

func TestRemotePermissionBoundsBodyBeforeParsing(t *testing.T) {
	for _, tc := range []struct {
		name     string
		length   int64
		wantRead int
	}{
		{"declared length", catalogs.MaxCatalogPermissionEnvelopeBytes + 1, 0},
		{"unknown length", -1, catalogs.MaxCatalogPermissionEnvelopeBytes + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &permissionCountedBody{Reader: strings.NewReader(strings.Repeat(" ", 2*catalogs.MaxCatalogPermissionEnvelopeBytes))}
			transport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{PermissionEnvelopeMediaType}}, ContentLength: tc.length, Body: body, Request: r}, nil
			})
			client, err := NewClient("http://127.0.0.1/api/v1", &http.Client{Transport: transport}, catalogs.CurrentCatalogSchemaVersion)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.FetchPermissionEnvelope(context.Background())
			if err == nil || !strings.Contains(err.Error(), "catalog_remote.body") || body.readBytes != tc.wantRead || !body.closed {
				t.Fatalf("body bound: error=%v, read=%d (want %d), closed=%v", err, body.readBytes, tc.wantRead, body.closed)
			}
		})
	}
}

type permissionCountedBody struct {
	*strings.Reader
	readBytes int
	closed    bool
}

func (b *permissionCountedBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.readBytes += n
	return n, err
}
func (b *permissionCountedBody) Close() error { b.closed = true; return nil }
