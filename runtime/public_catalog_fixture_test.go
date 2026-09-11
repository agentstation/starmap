package runtime

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogs/artifact"
)

//go:embed testdata/public-catalog-20260908/*
var signedPublicCatalog embed.FS

type publicCatalogFixture struct {
	server   *httptest.Server
	document artifact.Channel
	channel  []byte
	previous []byte
	assets   map[string][]byte
	bundles  map[string]json.RawMessage
	mu       sync.Mutex
	fault    string
	requests int
}

func newPublicCatalogFixture(t *testing.T) *publicCatalogFixture {
	t.Helper()
	read := func(name string) []byte {
		data, err := signedPublicCatalog.ReadFile("testdata/public-catalog-20260908/" + name)
		if errors.Is(err, fs.ErrNotExist) && name == artifact.Filename && os.Getenv("STARMAP_PUBLIC_FIXTURE_REQUIRED") != "1" {
			t.Skip("public catalog fixture is absent; run python3 scripts/prepare_public_catalog_fixture.py before testing")
		}
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	fixture := &publicCatalogFixture{
		channel: read("channel.json"), previous: read("previous-channel.json"),
		assets: make(map[string][]byte), bundles: make(map[string]json.RawMessage),
	}
	if err := json.Unmarshal(fixture.channel, &fixture.document); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{artifact.Filename, "starmap-catalog.tar.gz.sha256", "starmap-catalog.intoto.json"} {
		fixture.assets[name] = read(name)
	}
	for _, entry := range []struct {
		data []byte
		name string
	}{
		{fixture.channel, "channel-bundle.json"},
		{fixture.previous, "previous-channel-bundle.json"},
		{fixture.assets[artifact.Filename], "archive-bundle.json"},
	} {
		fixture.bundles[publicFixtureDigest(entry.data)] = read(entry.name)
	}
	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fixture.serve(t, w, r) }))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (f *publicCatalogFixture) setFault(fault string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fault = fault
}

func (f *publicCatalogFixture) requestCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests
}

func (f *publicCatalogFixture) serve(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	f.mu.Lock()
	fault := f.fault
	f.requests++
	f.mu.Unlock()
	if r.Header.Get("Authorization") != "" || r.Header.Get("X-API-Key") != "" {
		t.Error("public catalog request sent credentials")
	}
	if fault == "unavailable" {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	prefix := "/repos/agentstation/starmap/"
	var body []byte
	switch {
	case r.URL.Path == prefix+"contents/channel.json":
		if r.URL.Query().Get("ref") != artifact.ChannelName {
			t.Errorf("channel ref = %q", r.URL.Query().Get("ref"))
		}
		etag := `"public-` + fault + `"`
		w.Header().Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		body = f.channel
		if fault == "replay" {
			body = f.previous
		}
	case r.URL.Path == prefix+"releases/tags/"+f.document.Tag:
		assets := make([]map[string]any, 0, len(f.document.Assets))
		for _, asset := range f.document.Assets {
			size := len(f.assets[asset.Name])
			if fault == "size" && asset.Name == artifact.Filename {
				size++
			}
			assets = append(assets, map[string]any{"name": asset.Name, "size": size, "url": f.server.URL + "/assets/" + asset.Name})
		}
		body = publicFixtureJSON(t, map[string]any{"tag_name": f.document.Tag, "assets": assets})
	case strings.HasPrefix(r.URL.Path, "/assets/"):
		body = f.assets[strings.TrimPrefix(r.URL.Path, "/assets/")]
		if fault == "checksum" && r.URL.Path == "/assets/"+artifact.Filename {
			body = bytes.Clone(body)
			body[len(body)-1] ^= 1
		}
	case strings.HasPrefix(r.URL.Path, prefix+"attestations/sha256:"):
		digest := strings.TrimPrefix(r.URL.Path, prefix+"attestations/sha256:")
		bundle, found := f.bundles[digest]
		if !found {
			t.Errorf("unexpected attestation digest %q", digest)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if (fault == "channel signature" && digest == publicFixtureDigest(f.channel)) ||
			(fault == "archive signature" && digest == publicFixtureDigest(f.assets[artifact.Filename])) {
			bundle = corruptPublicFixtureSignature(t, bundle)
		}
		body = publicFixtureJSON(t, map[string]any{"attestations": []map[string]any{{"bundle": bundle}}})
	default:
		t.Errorf("unexpected catalog request %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if len(body) == 0 {
		t.Errorf("empty fixture response for %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if _, err := w.Write(body); err != nil {
		t.Errorf("write fixture response: %v", err)
	}
}

func publicFixtureDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func publicFixtureJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func corruptPublicFixtureSignature(t *testing.T, data []byte) json.RawMessage {
	t.Helper()
	var bundle map[string]any
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatal(err)
	}
	envelope := bundle["dsseEnvelope"].(map[string]any)
	signature := envelope["signatures"].([]any)[0].(map[string]any)
	value, err := base64.StdEncoding.DecodeString(signature["sig"].(string))
	if err != nil || len(value) == 0 {
		t.Fatalf("signature encoding: %v", err)
	}
	value[len(value)-1] ^= 1
	signature["sig"] = base64.StdEncoding.EncodeToString(value)
	return publicFixtureJSON(t, bundle)
}
