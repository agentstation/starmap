package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/application"
	"github.com/agentstation/starmap/internal/server/response"
)

func TestEmbeddedBudgetReadinessEndpointFailsWithStableReason(t *testing.T) {
	handler := &Handlers{app: &application.Mock{ReadinessFunc: func() (starmap.CatalogReadiness, error) {
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
