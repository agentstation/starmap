package config_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/runtime"
)

func TestOfflineCatalogLeavesCallerInferenceTransportAvailable(t *testing.T) {
	var catalogRequests, inferenceRequests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/catalog":
			catalogRequests.Add(1)
			w.WriteHeader(http.StatusNoContent)
		case "/inference":
			inferenceRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"output":"host inference accepted"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	client := server.Client()
	probe := &controlNetworkProbe{client: client, address: server.URL + "/catalog"}
	connected := openControlRuntime(t, map[string]string{config.NetworkMode: "offline"},
		runtime.WithSource(probe), runtime.WithAcquirer(probe))
	before := connected.State().GenerationID
	_, err := connected.Refresh(t.Context())
	assertConfiguredControlRefusal(t, "offline", err)
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/inference", strings.NewReader(`{"input":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("offline catalog mode blocked the caller's inference request: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || string(body) != `{"output":"host inference accepted"}` {
		t.Fatalf("caller inference response: status=%d body=%q error=%v", response.StatusCode, body, err)
	}
	if catalogRequests.Load() != 0 || inferenceRequests.Load() != 1 || connected.State().GenerationID != before {
		t.Fatal("catalog policy crossed the caller transport boundary or changed the serving catalog")
	}
}
