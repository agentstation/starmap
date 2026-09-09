package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/agentstation/starmap/internal/test/gitfixture"
	catalogconfig "github.com/agentstation/starmap/pkg/catalogs/config"
	"github.com/agentstation/starmap/pkg/sources"
)

type applicationMetadataFixture struct {
	id          sources.ID
	git         *gitfixture.Fixture
	temperature atomic.Bool
	calls       atomic.Int32
}

func newApplicationMetadataFixture(t *testing.T, id sources.ID) *applicationMetadataFixture {
	t.Helper()
	fixture := &applicationMetadataFixture{id: id}
	var payloads [][]byte
	for _, temperature := range []bool{false, true} {
		var payload map[string]any
		if err := json.Unmarshal(ingestionMetadataPayload(t), &payload); err != nil {
			t.Fatal(err)
		}
		model := payload["acme"].(map[string]any)["models"].(map[string]any)["known"].(map[string]any)
		model["temperature"], model["description"] = temperature, "Metadata fixture"
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		payloads = append(payloads, data)
	}
	var metadataURL *url.URL
	switch id {
	case sources.ModelsDevGitID:
		fixture.git = gitfixture.New(t, payloads...)
	case sources.ModelsDevHTTPID:
		metadataServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fixture.calls.Add(1)
			if r.URL.Path != "/api.json" {
				t.Errorf("metadata path %s", r.URL.Path)
			}
			index := 0
			if fixture.temperature.Load() {
				index = 1
			}
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write(payloads[index]); err != nil {
				t.Error(err)
			}
		}))
		t.Cleanup(metadataServer.Close)
		var err error
		metadataURL, err = url.Parse(metadataServer.URL)
		if err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unsupported metadata fixture source %s", id)
	}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = ingestionFixtureTransport(func(request *http.Request) (*http.Response, error) {
		if metadataURL != nil && request.URL.Hostname() == "models.dev" {
			copy := request.Clone(request.Context())
			copy.URL.Scheme, copy.URL.Host = metadataURL.Scheme, metadataURL.Host
			return originalTransport.RoundTrip(copy)
		}
		if request.URL.Hostname() != "127.0.0.1" && request.URL.Hostname() != "::1" {
			return nil, fmt.Errorf("unexpected external host %s", request.URL.Hostname())
		}
		return originalTransport.RoundTrip(request)
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	return fixture
}

func (f *applicationMetadataFixture) count(t *testing.T) int32 {
	t.Helper()
	if f.git != nil {
		return int32(f.git.BuildCount(t))
	}
	return f.calls.Load()
}

func (f *applicationMetadataFixture) commit() string {
	if f.git == nil {
		return ""
	}
	if f.temperature.Load() {
		return f.git.Commits[1]
	}
	return f.git.Commits[0]
}

func (f *applicationMetadataFixture) configure(values map[string]string) {
	if f.git == nil {
		return
	}
	values[catalogconfig.ModelsDevGitCommit] = f.commit()
	values[catalogconfig.AcquisitionSources] = "providers,local_catalog," + string(f.id)
}

func (f *applicationMetadataFixture) assertRevision(t *testing.T, revision sources.Revision) {
	t.Helper()
	if f.git == nil {
		return
	}
	if revision.Kind != sources.RevisionKindGitCommit || revision.Value != f.commit() || revision.InputName != "bun.lock" || revision.InputChecksum != f.git.LockfileChecksum {
		t.Fatalf("metadata lost pinned Git inputs: %+v", revision)
	}
}
