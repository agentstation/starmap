package runtime

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/pkg/catalogs"
	protocol "github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/remote"
)

func TestHTTPSUpstreamPreservesScopedEvidence(t *testing.T) {
	generation, observation, expected, _ := upstreamScopeFixture(t)
	manifest, err := protocol.MarshalManifest(generation.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1" + protocol.ManifestPath:
			w.Header().Set("Content-Type", protocol.ManifestMediaType)
			_, _ = w.Write(manifest)
		case "/api/v1" + protocol.PayloadPath(generation.Manifest.GenerationID):
			w.Header().Set("Content-Type", catalogs.CatalogPayloadMediaType)
			_, _ = w.Write(generation.Payload)
		case "/api/v1" + protocol.EventStreamPath:
			w.Header().Set("Content-Type", protocol.EventStreamMediaType)
			_, _ = fmt.Fprint(w, ": connected\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	for _, trusted := range []bool{false, true} {
		t.Run(fmt.Sprintf("trusted=%t", trusted), func(t *testing.T) {
			client := &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()}
			if trusted {
				client = server.Client()
			}
			defer client.CloseIdleConnections()
			source, err := remote.NewSource(t.Context(), remote.SourceConfig{Subscriber: remote.Config{
				BaseURL: server.URL + "/api/v1", HTTPClient: client, ShutdownTimeout: time.Second,
			}})
			if err != nil {
				t.Fatal(err)
			}
			defer source.Close()
			store := storage.NewMemory()
			runtime := openTestRuntime(t, WithSource(source), WithClientOptions(starmap.WithCatalogStore(store)))
			defer runtime.Close()
			before := runtime.State()
			_, err = runtime.RefreshSource(t.Context())
			if !trusted {
				if err == nil {
					t.Fatal("untrusted TLS publisher accepted")
				}
				if runtime.State().GenerationID != before.GenerationID || len(runtime.Catalog().MembershipScopes()) != 0 {
					t.Fatal("untrusted TLS publisher changed active catalog")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(runtime.Catalog().MembershipScopes(), expected) {
				t.Fatal("HTTPS import changed scope claims")
			}
			published, err := store.Current(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := catalogs.DecodeCatalogGeneration(published); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, link := range published.Manifest.SourceObservations {
				found = found || link == observation.Link()
			}
			if !found {
				t.Fatal("HTTPS import omitted the original scope receipt")
			}
		})
	}
}
