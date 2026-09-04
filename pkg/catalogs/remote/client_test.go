package remote

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
)

func TestNewClientSetsNoClientWideTimeout(t *testing.T) {
	client, err := NewClient("https://starmap.example.com/api/v1", nil, catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.httpClient == http.DefaultClient {
		t.Fatal("default HTTP client is the shared default client")
	}
	if client.httpClient.Timeout != 0 {
		t.Fatalf("default HTTP client timeout = %s, want no client-wide timeout", client.httpClient.Timeout)
	}
	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("default transport = %T, want *http.Transport", client.httpClient.Transport)
	}
	if transport.ResponseHeaderTimeout != DefaultResponseHeaderTimeout {
		t.Fatalf("response header timeout = %s, want %s",
			transport.ResponseHeaderTimeout, DefaultResponseHeaderTimeout)
	}
	if transport.TLSHandshakeTimeout != DefaultTLSHandshakeTimeout {
		t.Fatalf("TLS handshake timeout = %s, want %s",
			transport.TLSHandshakeTimeout, DefaultTLSHandshakeTimeout)
	}
}

func TestRemoteClientRejectsCrossOriginRedirect(t *testing.T) {
	var redirectedRequests atomic.Int32
	redirected := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirectedRequests.Add(1)
	}))
	defer redirected.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", redirected.URL)
		writer.WriteHeader(http.StatusFound)
	}))
	defer origin.Close()

	client, err := NewClient(origin.URL, origin.Client(), catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.FetchCurrent(context.Background()); err == nil {
		t.Fatal("FetchCurrent followed a cross-origin redirect")
	}
	if got := redirectedRequests.Load(); got != 0 {
		t.Fatalf("cross-origin requests = %d, want 0", got)
	}
}

func TestRemoteClientEnforcesConfiguredPublisherTrust(t *testing.T) {
	t.Parallel()

	credentialBearingURL := (&url.URL{
		Scheme: "https",
		User:   url.UserPassword("test-user", "test-password"),
		Host:   "catalog.example.com",
		Path:   "/api/v1",
	}).String()
	for _, baseURL := range []string{
		"http://catalog.example.com/api/v1",
		credentialBearingURL,
		"https://catalog.example.com/api/v1?channel=stable",
		"https://catalog.example.com/api/v1#current",
	} {
		if client, err := NewClient(
			baseURL,
			nil,
			catalogs.CurrentCatalogSchemaVersion,
		); err == nil {
			t.Fatalf("NewClient(%q) accepted ambiguous publisher %#v", baseURL, client)
		}
	}

	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{ManifestMediaType},
			},
			Body:    io.NopCloser(strings.NewReader("{}")),
			Request: request,
			// A custom transport cannot claim an HTTPS publisher without a
			// completed, verified certificate chain.
			TLS: nil,
		}, nil
	})
	client, err := NewClient(
		"https://catalog.example.com/api/v1",
		&http.Client{Transport: transport},
		catalogs.CurrentCatalogSchemaVersion,
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.FetchCurrent(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "publisher") {
		t.Fatalf("FetchCurrent publisher error = %v", err)
	}
}

func TestRemoteClientRequiresVerifiedTLSPublisherChain(t *testing.T) {
	t.Parallel()

	generation := remoteTestGeneration(
		t,
		catalogs.CurrentCatalogSchemaVersion,
		catalogs.ConsumerCompatibility{
			MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
			MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
		},
	)
	manifest, err := MarshalManifest(generation.Manifest)
	if err != nil {
		t.Fatalf("MarshalManifest: %v", err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case ManifestPath:
				writer.Header().Set("Content-Type", ManifestMediaType)
				_, _ = writer.Write(manifest)
			case PayloadPath(generation.Manifest.GenerationID):
				writer.Header().Set(
					"Content-Type",
					catalogs.CatalogPayloadMediaType,
				)
				_, _ = writer.Write(generation.Payload)
			default:
				http.NotFound(writer, request)
			}
		},
	))
	defer server.Close()

	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	verifiedHTTPClient := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs:    roots,
			MinVersion: tls.VersionTLS12,
		},
	}}
	verified, err := NewClient(
		server.URL,
		verifiedHTTPClient,
		catalogs.CurrentCatalogSchemaVersion,
	)
	if err != nil {
		t.Fatalf("NewClient verified: %v", err)
	}
	if _, err := verified.FetchCurrent(context.Background()); err != nil {
		t.Fatalf("FetchCurrent verified publisher: %v", err)
	}

	unverifiedHTTPClient := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // Negative publisher-trust fixture.
			MinVersion:         tls.VersionTLS12,
		},
	}}
	unverified, err := NewClient(
		server.URL,
		unverifiedHTTPClient,
		catalogs.CurrentCatalogSchemaVersion,
	)
	if err != nil {
		t.Fatalf("NewClient unverified: %v", err)
	}
	if _, err := unverified.FetchCurrent(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "publisher") {
		t.Fatalf("FetchCurrent unverified publisher error = %v", err)
	}
}

func TestFetchGenerationRequiresAddressedManifestAndPayload(t *testing.T) {
	t.Parallel()

	generation := remoteTestGeneration(
		t,
		catalogs.CurrentCatalogSchemaVersion,
		catalogs.ConsumerCompatibility{
			MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
			MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
		},
	)
	manifest, err := MarshalManifest(generation.Manifest)
	if err != nil {
		t.Fatalf("MarshalManifest: %v", err)
	}
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			paths = append(paths, request.URL.Path)
			switch request.URL.Path {
			case GenerationManifestPath(generation.Manifest.GenerationID):
				writer.Header().Set("Content-Type", ManifestMediaType)
				_, _ = writer.Write(manifest)
			case PayloadPath(generation.Manifest.GenerationID):
				writer.Header().Set("Content-Type", catalogs.CatalogPayloadMediaType)
				_, _ = writer.Write(generation.Payload)
			default:
				http.NotFound(writer, request)
			}
		},
	))
	defer server.Close()

	client, err := NewClient(
		server.URL,
		server.Client(),
		catalogs.CurrentCatalogSchemaVersion,
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	got, err := client.FetchGeneration(
		context.Background(),
		generation.Manifest.GenerationID,
	)
	if err != nil {
		t.Fatalf("FetchGeneration: %v", err)
	}
	if got.Manifest.GenerationID != generation.Manifest.GenerationID {
		t.Fatalf(
			"generation ID = %q, want %q",
			got.Manifest.GenerationID,
			generation.Manifest.GenerationID,
		)
	}
	wantPaths := []string{
		GenerationManifestPath(generation.Manifest.GenerationID),
		PayloadPath(generation.Manifest.GenerationID),
	}
	if !slices.Equal(paths, wantPaths) {
		t.Fatalf("request paths = %#v, want %#v", paths, wantPaths)
	}
}

func TestFetchCurrentIfChangedUsesConditionalManifest(t *testing.T) {
	t.Parallel()

	first := remoteTestGeneration(
		t,
		catalogs.CurrentCatalogSchemaVersion,
		catalogs.ConsumerCompatibility{
			MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
			MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
		},
	)
	second := first.Copy()
	second.Manifest.GenerationID += "-second"
	second.Manifest.GeneratedAt = second.Manifest.GeneratedAt.Add(time.Minute)
	if err := second.Validate(); err != nil {
		t.Fatalf("second generation: %v", err)
	}

	var (
		current       = first
		mu            sync.RWMutex
		manifestGets  atomic.Int32
		payloadGets   atomic.Int32
		ifNoneMatches []string
	)
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			mu.RLock()
			selected := current
			mu.RUnlock()
			switch request.URL.Path {
			case ManifestPath:
				manifestGets.Add(1)
				mu.Lock()
				ifNoneMatches = append(
					ifNoneMatches,
					request.Header.Get("If-None-Match"),
				)
				mu.Unlock()
				etag := ManifestETag(selected.Manifest.GenerationID)
				writer.Header().Set("ETag", etag)
				if request.Header.Get("If-None-Match") == etag {
					writer.WriteHeader(http.StatusNotModified)
					return
				}
				data, err := MarshalManifest(selected.Manifest)
				if err != nil {
					t.Fatalf("MarshalManifest: %v", err)
				}
				writer.Header().Set("Content-Type", ManifestMediaType)
				_, _ = writer.Write(data)
			case PayloadPath(selected.Manifest.GenerationID):
				payloadGets.Add(1)
				writer.Header().Set(
					"Content-Type",
					catalogs.CatalogPayloadMediaType,
				)
				_, _ = writer.Write(selected.Payload)
			default:
				http.NotFound(writer, request)
			}
		},
	))
	defer server.Close()

	client, err := NewClient(
		server.URL,
		server.Client(),
		catalogs.CurrentCatalogSchemaVersion,
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	generation, changed, err := client.FetchCurrentIfChanged(
		context.Background(),
		first.Manifest.GenerationID,
	)
	if err != nil || changed || generation.Manifest.GenerationID != "" {
		t.Fatalf("unchanged fetch = %#v/%t/%v", generation, changed, err)
	}
	if got := payloadGets.Load(); got != 0 {
		t.Fatalf("unchanged payload GETs = %d, want 0", got)
	}

	mu.Lock()
	current = second
	mu.Unlock()
	generation, changed, err = client.FetchCurrentIfChanged(
		context.Background(),
		first.Manifest.GenerationID,
	)
	if err != nil || !changed {
		t.Fatalf("changed fetch = %#v/%t/%v", generation, changed, err)
	}
	if generation.Manifest.GenerationID != second.Manifest.GenerationID {
		t.Fatalf(
			"changed generation = %q, want %q",
			generation.Manifest.GenerationID,
			second.Manifest.GenerationID,
		)
	}
	if got := payloadGets.Load(); got != 1 {
		t.Fatalf("changed payload GETs = %d, want 1", got)
	}
	mu.RLock()
	gotMatches := append([]string(nil), ifNoneMatches...)
	mu.RUnlock()
	wantETag := ManifestETag(first.Manifest.GenerationID)
	if !slices.Equal(gotMatches, []string{wantETag, wantETag}) {
		t.Fatalf("If-None-Match values = %#v, want repeated %q", gotMatches, wantETag)
	}
	if got := manifestGets.Load(); got != 2 {
		t.Fatalf("manifest GETs = %d, want 2", got)
	}
}

func TestFetchGenerationRejectsAddressedManifestIdentityMismatch(t *testing.T) {
	t.Parallel()

	generation := remoteTestGeneration(
		t,
		catalogs.CurrentCatalogSchemaVersion,
		catalogs.ConsumerCompatibility{
			MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
			MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion,
		},
	)
	manifest, err := MarshalManifest(generation.Manifest)
	if err != nil {
		t.Fatalf("MarshalManifest: %v", err)
	}
	var (
		requests    atomic.Int32
		payloadGets atomic.Int32
	)
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			requests.Add(1)
			switch request.URL.Path {
			case GenerationManifestPath("requested-generation"):
				writer.Header().Set("Content-Type", ManifestMediaType)
				_, _ = writer.Write(manifest)
			case PayloadPath(generation.Manifest.GenerationID):
				payloadGets.Add(1)
				http.Error(writer, "must not fetch", http.StatusInternalServerError)
			default:
				http.NotFound(writer, request)
			}
		},
	))
	defer server.Close()

	client, err := NewClient(
		server.URL,
		server.Client(),
		catalogs.CurrentCatalogSchemaVersion,
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.FetchGeneration(
		context.Background(),
		"../other-generation",
	); err == nil {
		t.Fatal("FetchGeneration accepted a noncanonical path identity")
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("requests after unsafe identity = %d, want 0", got)
	}
	if _, err := client.FetchGeneration(
		context.Background(),
		"requested-generation",
	); err == nil {
		t.Fatal("FetchGeneration accepted a manifest for another generation")
	}
	if got := payloadGets.Load(); got != 0 {
		t.Fatalf("payload GETs after identity mismatch = %d, want 0", got)
	}
}

func TestRemoteCatalogFetchValidatesManifestPayloadChecksumAndCompatibility(t *testing.T) {
	current := catalogs.CurrentCatalogSchemaVersion
	valid := remoteTestGeneration(t, current, catalogs.ConsumerCompatibility{
		MinSchemaVersion: current,
		MaxSchemaVersion: current,
	})
	wrongDescriptorMedia := valid.Copy()
	wrongDescriptorMedia.Manifest.Payload.MediaType = "application/json"
	oversizedDescriptor := valid.Copy()
	oversizedDescriptor.Manifest.Payload.SizeBytes = maxBodyBytes + 1
	wrongDescriptorSize := valid.Copy()
	wrongDescriptorSize.Manifest.Payload.SizeBytes++
	wrongDescriptorChecksum := valid.Copy()
	wrongDescriptorChecksum.Manifest.Payload.Checksum =
		"sha256:" + strings.Repeat("0", 64)
	unsafeGenerationID := valid.Copy()
	unsafeGenerationID.Manifest.GenerationID = ".."
	for _, test := range []struct {
		name                 string
		generation           catalogs.Generation
		mutatePayload        func([]byte) []byte
		manifestType         string
		payloadType          string
		payloadContentLength int64
		wantError            bool
		wantPayloadGet       bool
	}{
		{name: "valid", generation: valid, manifestType: ManifestMediaType, payloadType: catalogs.CatalogPayloadMediaType, wantPayloadGet: true},
		{name: "corrupt payload", generation: valid, mutatePayload: func(data []byte) []byte {
			copyData := append([]byte(nil), data...)
			copyData[len(copyData)-1] ^= 1
			return copyData
		}, manifestType: ManifestMediaType, payloadType: catalogs.CatalogPayloadMediaType, wantError: true, wantPayloadGet: true},
		{name: "wrong manifest media type", generation: valid, manifestType: "application/json", payloadType: catalogs.CatalogPayloadMediaType, wantError: true},
		{name: "wrong payload media type", generation: valid, manifestType: ManifestMediaType, payloadType: "application/json", wantError: true, wantPayloadGet: true},
		{name: "wrong descriptor media type before payload", generation: wrongDescriptorMedia, manifestType: ManifestMediaType, payloadType: catalogs.CatalogPayloadMediaType, wantError: true},
		{name: "oversized descriptor before payload", generation: oversizedDescriptor, manifestType: ManifestMediaType, payloadType: catalogs.CatalogPayloadMediaType, wantError: true},
		{name: "wrong descriptor size", generation: wrongDescriptorSize, manifestType: ManifestMediaType, payloadType: catalogs.CatalogPayloadMediaType, wantError: true, wantPayloadGet: true},
		{name: "wrong descriptor checksum", generation: wrongDescriptorChecksum, manifestType: ManifestMediaType, payloadType: catalogs.CatalogPayloadMediaType, wantError: true, wantPayloadGet: true},
		{name: "unsafe generation ID before payload", generation: unsafeGenerationID, manifestType: ManifestMediaType, payloadType: catalogs.CatalogPayloadMediaType, wantError: true},
		{
			name:                 "oversized response before body read",
			generation:           valid,
			manifestType:         ManifestMediaType,
			payloadType:          catalogs.CatalogPayloadMediaType,
			payloadContentLength: maxBodyBytes + 1,
			wantError:            true,
			wantPayloadGet:       true,
		},
		{
			name: "incompatible before payload",
			generation: remoteTestGeneration(t, current+1, catalogs.ConsumerCompatibility{
				MinSchemaVersion: current + 1,
				MaxSchemaVersion: current + 1,
			}),
			manifestType: ManifestMediaType, payloadType: catalogs.CatalogPayloadMediaType, wantError: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifest, err := MarshalManifest(test.generation.Manifest)
			if err != nil {
				t.Fatalf("MarshalManifest: %v", err)
			}
			var payloadGets atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch request.URL.Path {
				case ManifestPath:
					writer.Header().Set("Content-Type", test.manifestType)
					_, _ = writer.Write(manifest)
				case PayloadPath(test.generation.Manifest.GenerationID):
					payloadGets.Add(1)
					writer.Header().Set("Content-Type", test.payloadType)
					if test.payloadContentLength > 0 {
						writer.Header().Set(
							"Content-Length",
							fmt.Sprint(test.payloadContentLength),
						)
						writer.WriteHeader(http.StatusOK)
						return
					}
					payload := test.generation.Payload
					if test.mutatePayload != nil {
						payload = test.mutatePayload(payload)
					}
					_, _ = writer.Write(payload)
				default:
					http.NotFound(writer, request)
				}
			}))
			defer server.Close()
			client, err := NewClient(server.URL, server.Client(), catalogs.CurrentCatalogSchemaVersion)
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			got, err := client.FetchCurrent(context.Background())
			if (err != nil) != test.wantError {
				t.Fatalf("FetchCurrent = %#v/%v", got, err)
			}
			if (payloadGets.Load() > 0) != test.wantPayloadGet {
				t.Fatalf("payload GETs = %d, want requested=%t", payloadGets.Load(), test.wantPayloadGet)
			}
			if err == nil && got.Manifest.GenerationID != valid.Manifest.GenerationID {
				t.Fatalf("generation ID = %q", got.Manifest.GenerationID)
			}
		})
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func remoteTestGeneration(t *testing.T, schemaVersion uint64, compatibility catalogs.ConsumerCompatibility) catalogs.Generation {
	t.Helper()
	builder := catalogs.NewEmpty()
	if err := builder.SetProvider(catalogs.Provider{ID: "remote-test", Name: "Remote Test"}); err != nil {
		t.Fatalf("SetProvider: %v", err)
	}
	catalog, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	payload, err := catalogs.EncodeCatalogPayload(catalog)
	if err != nil {
		t.Fatalf("EncodeCatalogPayload: %v", err)
	}
	descriptor := catalogs.DescribeCatalogPayload(payload)
	generatedAt := time.Date(2026, time.July, 11, 3, 0, 0, 0, time.UTC)
	generation := catalogs.Generation{
		Manifest: catalogs.GenerationManifest{
			ManifestVersion: catalogs.CurrentGenerationManifestVersion,
			SchemaVersion:   schemaVersion, GenerationID: fmt.Sprintf("remote-generation-%d", schemaVersion),
			GeneratedAt: generatedAt, Payload: descriptor,
			Validation: catalogs.GenerationValidationReport{
				ValidatorVersion: "remote-test/v1", ValidatedAt: generatedAt, Status: catalogs.GenerationValidationPassed,
				Checks: []catalogs.GenerationValidationCheck{{Name: "test", Status: catalogs.GenerationValidationCheckPassed}},
			},
			SyncRunID: "remote-sync-run",
			SourceObservations: []catalogs.SourceObservationLink{{
				Source: evidence.LocalCatalogID, ObservationID: "remote-observation", ObservedAt: generatedAt,
				Revision:     evidence.ObservationRevision{Kind: evidence.ObservationRevisionKindContentDigest, Value: descriptor.Checksum},
				Completeness: evidence.ObservationCompletenessComplete, Status: evidence.ObservationStatusSucceeded,
				EvidenceChecksum: descriptor.Checksum,
			}},
			ReviewCandidates: []evidence.ReviewCandidate{},
			Completeness:     catalogs.GenerationCompletenessComplete, ConsumerCompatibility: compatibility,
		},
		Payload: payload,
	}
	if err := generation.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return generation
}
