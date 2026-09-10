package remote

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
)

func TestAuthorityObserverPrecedesPayloadCompatibilityAndTransfer(t *testing.T) {
	for _, test := range []struct {
		name                                                           string
		unknownSchema, ordinary, invalid, refuse, untrusted, addressed bool
		wantObserver, wantPayload                                      int32
	}{
		{name: "current authority", wantObserver: 1, wantPayload: 1},
		{name: "addressed history", addressed: true, wantPayload: 1},
		{name: "unsupported schema", unknownSchema: true, wantObserver: 1},
		{name: "ordinary manifest", ordinary: true, refuse: true, wantObserver: 1},
		{name: "retention refusal", refuse: true, wantObserver: 1},
		{name: "invalid binding", invalid: true},
		{name: "untrusted publisher", untrusted: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			generation := remoteTestGeneration(t, catalogs.CurrentCatalogSchemaVersion, catalogs.ConsumerCompatibility{MinSchemaVersion: catalogs.CurrentCatalogSchemaVersion, MaxSchemaVersion: catalogs.CurrentCatalogSchemaVersion})
			if !test.ordinary {
				generation.Manifest.ManifestVersion = catalogs.AuthorityGenerationManifestVersion
				generation.Manifest.AuthorityHead = catalogs.CatalogAuthorityHead{AuthorityID: "enterprise", PolicyID: "production", Sequence: 2, GenerationID: generation.Manifest.GenerationID, PayloadChecksum: generation.Manifest.Payload.Checksum, RequiredPermissionRevision: "sha256:" + strings.Repeat("a", 64), PermissionSchemaVersion: catalogs.CatalogPermissionSchemaVersion}
			}
			if test.unknownSchema {
				generation.Manifest.SchemaVersion++
				generation.Manifest.ConsumerCompatibility = catalogs.ConsumerCompatibility{MinSchemaVersion: generation.Manifest.SchemaVersion, MaxSchemaVersion: generation.Manifest.SchemaVersion}
			}
			if test.invalid {
				generation.Manifest.AuthorityHead.PayloadChecksum = "sha256:" + strings.Repeat("b", 64)
			}
			data, err := json.Marshal(generation.Manifest)
			if err != nil {
				t.Fatal(err)
			}
			var observed, payloads atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == ManifestPath || r.URL.Path == GenerationManifestPath(generation.Manifest.GenerationID) {
					w.Header().Set("Content-Type", ManifestMediaType)
					_, _ = w.Write(data)
					return
				}
				payloads.Add(1)
				w.Header().Set("Content-Type", catalogs.CatalogPayloadMediaType)
				_, _ = w.Write(generation.Payload)
			}))
			defer server.Close()
			httpClient := server.Client()
			if test.untrusted {
				httpClient = &http.Client{}
			}
			client, err := NewClient(server.URL, httpClient, catalogs.CurrentCatalogSchemaVersion)
			if err != nil {
				t.Fatal(err)
			}
			refusal := &errors.ConflictError{Resource: "test permission retention", Message: "refused"}
			if err := client.BindAuthorityObserver(func(_ context.Context, head catalogs.CatalogAuthorityHead) error {
				observed.Add(1)
				if head != generation.Manifest.AuthorityHead {
					t.Error("observer received a different authority head")
				}
				if payloads.Load() != 0 {
					t.Error("observer ran after payload transfer")
				}
				if test.refuse {
					return refusal
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if test.addressed {
				_, err = client.FetchGeneration(t.Context(), generation.Manifest.GenerationID)
			} else {
				_, err = client.FetchCurrent(t.Context())
			}
			if (err == nil) != (test.wantPayload == 1) {
				t.Fatalf("unexpected fetch result: %v", err)
			}
			if test.refuse && err != refusal {
				t.Fatalf("observer refusal identity changed: %v", err)
			}
			if observed.Load() != test.wantObserver || payloads.Load() != test.wantPayload {
				t.Fatalf("observer=%d payload=%d", observed.Load(), payloads.Load())
			}
		})
	}
}

func TestAuthorityObserverBindsOnceBeforeAnyManifestRequest(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); http.NotFound(w, r) }))
	defer server.Close()
	client, err := NewClient(server.URL, server.Client(), catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.BindAuthorityObserver(nil); err == nil {
		t.Fatal("nil observer accepted")
	}
	observer := func(context.Context, catalogs.CatalogAuthorityHead) error { return nil }
	var successful atomic.Int32
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			if client.BindAuthorityObserver(observer) == nil {
				successful.Add(1)
			}
		})
	}
	group.Wait()
	if successful.Load() != 1 || requests.Load() != 0 {
		t.Fatal("binding was not unique and passive")
	}
	other, err := NewClient(server.URL, server.Client(), catalogs.CurrentCatalogSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.FetchCurrent(t.Context()); err == nil {
		t.Fatal("missing manifest accepted")
	}
	if err := other.BindAuthorityObserver(observer); err == nil {
		t.Fatal("observer bound after a manifest request")
	}
}
