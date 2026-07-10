package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentstation/starmap/internal/application"
	"github.com/agentstation/starmap/pkg/catalogscheduler"
	"github.com/agentstation/starmap/pkg/types"
)

func TestOperationsEndpointExposesGenerationFreshnessLastSyncDegradationAndScheduler(t *testing.T) {
	state := catalogscheduler.OperationalState{
		EvaluatedAt: time.Date(2026, time.July, 10, 12, 0, 0, 0, time.UTC),
		Catalog:     catalogscheduler.CatalogIdentity{GenerationID: "generation-1", Sequence: 9},
		Freshness:   &catalogscheduler.FreshnessReport{Ready: true, Degraded: true},
		LastSync: &catalogscheduler.OperationalRun{
			ID: "run-1", Trigger: catalogscheduler.TriggerScheduled,
			Status: catalogscheduler.RunStatusSucceeded, SyncRunID: "sync-1",
		},
		DegradedSources: []types.SourceID{types.ModelsDevHTTPID},
		Scheduler:       catalogscheduler.SchedulerOperationalState{Configured: true},
	}
	handler := &Handlers{app: &application.Mock{OperationalStateFunc: func(context.Context) (catalogscheduler.OperationalState, error) {
		return state, nil
	}}}
	recorder := httptest.NewRecorder()
	handler.HandleOperations(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/operations", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data catalogscheduler.OperationalState `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if body.Data.Catalog.GenerationID != "generation-1" || body.Data.Catalog.Sequence != 9 ||
		body.Data.Freshness == nil || !body.Data.Freshness.Degraded ||
		body.Data.LastSync == nil || body.Data.LastSync.ID != "run-1" ||
		len(body.Data.DegradedSources) != 1 || !body.Data.Scheduler.Configured {
		t.Fatalf("response state = %#v", body.Data)
	}
}
