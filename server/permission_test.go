package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/embedded/openapi"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/runtime/status"
)

func TestServerPermissionOpenAPIUsesTheWireMediaType(t *testing.T) {
	var spec struct {
		Paths map[string]struct {
			Get struct {
				Responses map[string]struct {
					Content map[string]struct {
						Schema struct {
							Ref string `json:"$ref"`
						} `json:"schema"`
					} `json:"content"`
				} `json:"responses"`
			} `json:"get"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(openapi.SpecJSON, &spec); err != nil {
		t.Fatal(err)
	}
	content := spec.Paths["/api/v1/catalog/permission"].Get.Responses["200"].Content
	if len(content) != 1 || content[remote.PermissionEnvelopeMediaType].Schema.Ref != "#/components/schemas/catalogs.CatalogPermissionEnvelope" {
		t.Fatalf("permission OpenAPI does not match the wire response: %+v", content)
	}
}

type receiptRuntime struct {
	receipt catalogs.CatalogPermissionEnvelope
	err     error
	reads   int
}

func TestServerPermissionRefusalAndAuthentication(t *testing.T) {
	client, err := starmap.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("API_KEY", "permission-relay-test-key")
	for _, test := range []struct {
		name      string
		connected *receiptRuntime
		method    string
		auth      bool
		key       string
		want      int
	}{
		{name: "absent reader", method: http.MethodGet, want: http.StatusServiceUnavailable},
		{name: "unavailable receipt", connected: &receiptRuntime{err: &errors.ConfigError{Component: "secret-sentinel", Message: "not for responses"}}, method: http.MethodGet, want: http.StatusServiceUnavailable},
		{name: "invalid envelope", connected: &receiptRuntime{}, method: http.MethodGet, want: http.StatusServiceUnavailable},
		{name: "method", connected: &receiptRuntime{receipt: serverReceipt()}, method: http.MethodPost, want: http.StatusMethodNotAllowed},
		{name: "missing key", connected: &receiptRuntime{receipt: serverReceipt()}, method: http.MethodGet, auth: true, want: http.StatusUnauthorized},
		{name: "wrong key", connected: &receiptRuntime{receipt: serverReceipt()}, method: http.MethodGet, auth: true, key: "wrong", want: http.StatusUnauthorized},
		{name: "configured key", connected: &receiptRuntime{receipt: serverReceipt()}, method: http.MethodGet, auth: true, key: "permission-relay-test-key", want: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := DefaultConfig()
			config.AuthEnabled = test.auth
			var options []Option
			if test.connected != nil {
				options = append(options, WithRuntime(test.connected))
			}
			srv, err := New(client, config, options...)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(test.method, "/api/v1/catalog/permission", nil)
			request.Header.Set(config.AuthHeader, test.key)
			recorder := httptest.NewRecorder()
			srv.Handler().ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", recorder.Code, test.want, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), "secret-sentinel") {
				t.Fatal("receipt error leaked private details")
			}
			if test.want == http.StatusUnauthorized || test.want == http.StatusMethodNotAllowed {
				if test.connected.reads != 0 {
					t.Fatal("rejected request read a permission receipt")
				}
			} else if recorder.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("permission response permits caching")
			}
		})
	}
}

func (r *receiptRuntime) ReadPermission(context.Context) (catalogs.CatalogPermissionEnvelope, error) {
	r.reads++
	return r.receipt, r.err
}
func (*receiptRuntime) Status() status.Status { return status.Status{} }
func (*receiptRuntime) Close() error          { return nil }

func serverReceipt() catalogs.CatalogPermissionEnvelope {
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	return catalogs.CatalogPermissionEnvelope{Version: catalogs.CatalogPermissionEnvelopeVersion,
		Head: catalogs.CatalogAuthorityHead{AuthorityID: "enterprise", PolicyID: "production", Sequence: 1,
			GenerationID: "approved-one", PayloadChecksum: "sha256:" + strings.Repeat("a", 64),
			RequiredPermissionRevision: "sha256:" + strings.Repeat("b", 64), PermissionSchemaVersion: catalogs.CatalogPermissionSchemaVersion},
		IssuedAt: at, ValidUntil: at.Add(time.Minute)}
}

func TestServerPermissionReturnsExactReceiptWithoutConditionalCaching(t *testing.T) {
	client, err := starmap.New()
	if err != nil {
		t.Fatal(err)
	}
	r := &receiptRuntime{receipt: serverReceipt()}
	srv, err := New(client, DefaultConfig(), WithRuntime(r))
	if err != nil {
		t.Fatal(err)
	}
	if r.reads != 0 {
		t.Fatal("server construction read permission")
	}
	for _, conditional := range []string{"", "*", `"approved-one"`} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/permission", nil)
		request.Header.Set("If-None-Match", conditional)
		request.Header.Set("If-Modified-Since", r.receipt.IssuedAt.Format(http.TimeFormat))
		recorder := httptest.NewRecorder()
		srv.Handler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("permission status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		got, err := catalogs.ParseCatalogPermissionEnvelope(recorder.Body.Bytes())
		if err != nil || got != r.receipt {
			t.Fatalf("receipt changed: %+v %v", got, err)
		}
		if recorder.Header().Get("Content-Type") != remote.PermissionEnvelopeMediaType || recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("ETag") != "" {
			t.Fatalf("unsafe permission headers: %v", recorder.Header())
		}
	}
	if r.reads != 3 {
		t.Fatalf("permission reads=%d, want 3", r.reads)
	}
}
