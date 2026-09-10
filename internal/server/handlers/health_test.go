package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/server/response"
	"github.com/agentstation/starmap/runtime"
)

func TestEmbeddedBudgetReadinessEndpointFailsWithStableReason(t *testing.T) {
	handler := &Handlers{app: &testApplication{ReadinessFunc: func() (starmap.CatalogReadiness, error) {
		return starmap.CatalogReadiness{
			Issues: []starmap.ReadinessIssue{{
				Code:    starmap.ReadinessIssueEmbeddedBootstrapStale,
				Message: "embedded bootstrap exceeds configured maximum age",
			}},
		}, nil
	}}}
	recorder := httptest.NewRecorder()
	handler.HandleReady(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	var body response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal response: %v", err)
	}
	if body.Error == nil || !strings.Contains(body.Error.Details, starmap.ReadinessIssueEmbeddedBootstrapStale) {
		t.Fatalf("response = %#v", body)
	}
}

func TestAuthorityReadinessRefusesNewWorkAndKeepsDiagnostics(t *testing.T) {
	handler := &Handlers{app: &testApplication{RuntimeFunc: func() runtime.Status {
		return runtime.Status{AuthorityRequired: true, CatalogAvailable: true, RequiredPermissionRevision: "required-revision"}
	}}}
	recorder := httptest.NewRecorder()
	handler.HandleReady(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness status = %d", recorder.Code)
	}
	var body struct {
		Data struct {
			Runtime struct {
				CatalogAvailable           bool   `json:"catalog_available"`
				RequiredPermissionRevision string `json:"required_permission_revision"`
			} `json:"runtime"`
		} `json:"data"`
		Error *response.Error `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error == nil || body.Error.Code != "catalog_authority_unavailable" || !body.Data.Runtime.CatalogAvailable || body.Data.Runtime.RequiredPermissionRevision != "required-revision" {
		t.Fatal("readiness discarded the authority diagnostics")
	}
	liveness := httptest.NewRecorder()
	handler.HandleHealth(liveness, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if liveness.Code != http.StatusOK {
		t.Fatal("authority refusal disabled liveness")
	}
}
