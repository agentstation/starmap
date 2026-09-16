package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
	"github.com/agentstation/starmap/server/administration"
)

type administrativeSyncer struct{ calls atomic.Int32 }

func (s *administrativeSyncer) Sync(context.Context, ...pkgsync.Option) (*pkgsync.Result, error) {
	s.calls.Add(1)
	return &pkgsync.Result{}, nil
}

func TestSubscriberCannotStartAdministrativeUpdate(t *testing.T) {
	t.Setenv("API_KEY", "synthetic-legacy-reader")
	client, err := starmap.New()
	if err != nil {
		t.Fatal(err)
	}
	manager, adminToken, err := administration.Initialize(t.Context(), administration.Config{StateDirectory: filepath.Join(t.TempDir(), "state"), Audience: "enterprise"}, "operator")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	actor, _ := manager.Authenticate(adminToken, "enterprise")
	subscriberToken, err := manager.Create(t.Context(), actor, "gateway", administration.Subscriber)
	if err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.RateLimit = 0
	cfg.AuthEnabled = true
	syncer := &administrativeSyncer{}
	srv, err := New(client, cfg, WithAdministration(manager, "enterprise"), WithSyncer(syncer))
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown(context.Background())
	for _, test := range []struct {
		name, token, method, path string
		want                      int
	}{
		{"subscriber reads", subscriberToken, http.MethodGet, "/api/v1/providers", 200},
		{"subscriber mutation", subscriberToken, http.MethodPost, "/api/v1/update", 403},
		{"legacy reader mutation", "synthetic-legacy-reader", http.MethodPost, "/api/v1/update", 401},
		{"inference key mutation", "synthetic-starport-inference", http.MethodPost, "/api/v1/update", 401},
		{"anonymous mutation", "", http.MethodPost, "/api/v1/update", 401},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			request.Header.Set("Authorization", "Bearer "+test.token)
			response := httptest.NewRecorder()
			srv.Handler().ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d", response.Code, test.want)
			}
			if syncer.calls.Load() != 0 {
				t.Fatal("reader request started acquisition")
			}
		})
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/update", nil)
	request.Header.Set("Authorization", "Bearer "+adminToken)
	response := httptest.NewRecorder()
	srv.Handler().ServeHTTP(response, request)
	if response.Code != 202 {
		t.Fatalf("administrator update status=%d", response.Code)
	}
	var accepted struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		receipt, err := manager.Operation(actor, accepted.Data.ID)
		if err != nil {
			t.Fatal(err)
		}
		if receipt.State == administration.Succeeded {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("administrative outcome was not persisted")
		}
		time.Sleep(time.Millisecond)
	}
	if syncer.calls.Load() != 1 {
		t.Fatal("administrator update did not run exactly once")
	}
}

func TestAdministratorHTTPRotationAndRevocation(t *testing.T) {
	client, err := starmap.New()
	if err != nil {
		t.Fatal(err)
	}
	manager, adminToken, err := administration.Initialize(t.Context(), administration.Config{StateDirectory: filepath.Join(t.TempDir(), "state"), Audience: "test"}, "operator")
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	cfg := DefaultConfig()
	cfg.RateLimit = 0
	srv, err := New(client, cfg, WithAdministration(manager, "test"))
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, token, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		srv.Handler().ServeHTTP(response, req)
		if response.Code != want {
			t.Fatalf("%s %s: status=%d want=%d", method, path, response.Code, want)
		}
		return response
	}
	credential := func(response *httptest.ResponseRecorder) string {
		t.Helper()
		var result struct {
			Credential string `json:"credential"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Credential == "" {
			t.Fatal("identity response has no credential")
		}
		if response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("credential response can be cached")
		}
		return result.Credential
	}
	old := credential(request(http.MethodPost, "/admin/identities", adminToken, `{"id":"gateway","role":"subscriber"}`, 201))
	request(http.MethodPost, "/admin/identities/gateway/rotate", old, `{"overlap":"0s"}`, 403)
	next := credential(request(http.MethodPost, "/admin/identities/gateway/rotate", adminToken, `{"overlap":"0s"}`, 200))
	request(http.MethodGet, "/api/v1/providers", old, "", 401)
	request(http.MethodGet, "/api/v1/providers", next, "", 200)
	request(http.MethodDelete, "/admin/identities/gateway", adminToken, "", 204)
	request(http.MethodGet, "/api/v1/providers", next, "", 401)
	request(http.MethodDelete, "/admin/identities/operator", adminToken, "", 409)
	request(http.MethodPost, "/admin/identities", adminToken, `{"id":"unknown","role":"subscriber","unrecognized":true}`, 400)
	request(http.MethodPost, "/admin/identities", adminToken, strings.Repeat(" ", 5000)+`{}`, 400)
}
