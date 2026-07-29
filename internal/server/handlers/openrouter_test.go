package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/server/openrouter"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

func TestHandleOpenRouterModelUsesNumericInternalErrorEnvelope(t *testing.T) {
	t.Parallel()

	h := &Handlers{app: &testApplication{
		CatalogStateFunc: func() (starmap.CatalogState, error) {
			return starmap.CatalogState{}, &pkgerrors.ConfigError{
				Component: "catalog", Message: "unavailable",
			}
		},
	}}
	recorder := httptest.NewRecorder()
	h.HandleOpenRouterModel(
		recorder,
		httptest.NewRequest(http.MethodGet, "/api/v1/model/lab/model", nil),
		"lab",
		"model",
		"/api/v1",
	)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status = %d, want %d: %s",
			recorder.Code,
			http.StatusInternalServerError,
			recorder.Body.String(),
		)
	}
	var envelope openrouter.ErrorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Error.Code != http.StatusInternalServerError ||
		envelope.Error.Message != "Internal Server Error" {
		t.Fatalf("error envelope = %#v", envelope)
	}
}
