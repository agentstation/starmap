package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentstation/starmap/internal/server/operations"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

// TestAdminUpdateReturnsAcceptedOperation proves the admin update contract. The
// endpoint accepts the work and names an operation. A later status read reports
// progress and completion, and a client can cancel a running operation.
func TestAdminUpdateReturnsAcceptedOperation(t *testing.T) {
	t.Run("completes", func(t *testing.T) {
		release := make(chan struct{})
		started := make(chan struct{})
		handlers, registry := newOperationHandlers(t, func(
			ctx context.Context,
		) (*pkgsync.Result, error) {
			close(started)
			select {
			case <-release:
				return &pkgsync.Result{
					TotalChanges: 3,
					GenerationID: "generation-accepted",
				}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})

		accepted := postUpdate(t, handlers)
		if accepted.ID == "" {
			t.Fatal("the accepted operation carried no identity")
		}
		if accepted.Kind != operations.KindCatalogUpdate {
			t.Fatalf("kind = %q, want %q", accepted.Kind, operations.KindCatalogUpdate)
		}
		if accepted.State != operations.StateAccepted {
			t.Fatalf("state = %q, want %q", accepted.State, operations.StateAccepted)
		}

		<-started
		running := readOperation(t, handlers, accepted.ID, http.StatusOK)
		if running.State != operations.StateRunning {
			t.Fatalf("state during the run = %q, want %q", running.State, operations.StateRunning)
		}

		close(release)
		<-registry.Done(accepted.ID)
		final := readOperation(t, handlers, accepted.ID, http.StatusOK)
		if final.State != operations.StateSucceeded {
			t.Fatalf("final state = %q, want %q", final.State, operations.StateSucceeded)
		}
		if final.Detail["generation_id"] != "generation-accepted" {
			t.Fatalf("detail = %#v, want the published generation", final.Detail)
		}
		if final.CompletedAt.Before(final.StartedAt) {
			t.Fatalf("completion %s precedes the start %s", final.CompletedAt, final.StartedAt)
		}
	})

	t.Run("cancels", func(t *testing.T) {
		started := make(chan struct{})
		handlers, registry := newOperationHandlers(t, func(
			ctx context.Context,
		) (*pkgsync.Result, error) {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		})

		accepted := postUpdate(t, handlers)
		<-started

		recorder := httptest.NewRecorder()
		handlers.HandleOperationCancel(
			recorder,
			httptest.NewRequest(http.MethodDelete, "/api/v1/updates/"+accepted.ID, nil),
			accepted.ID,
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("cancel status = %d, want 200: %s", recorder.Code, recorder.Body)
		}

		<-registry.Done(accepted.ID)
		final := readOperation(t, handlers, accepted.ID, http.StatusOK)
		if final.State != operations.StateCanceled {
			t.Fatalf("final state = %q, want %q", final.State, operations.StateCanceled)
		}
		if final.Reason != "" {
			t.Fatalf("reason = %q, want none for a cancellation", final.Reason)
		}
	})

	t.Run("reports an unknown operation", func(t *testing.T) {
		handlers, _ := newOperationHandlers(t, func(
			context.Context,
		) (*pkgsync.Result, error) {
			return &pkgsync.Result{}, nil
		})
		recorder := httptest.NewRecorder()
		handlers.HandleOperationStatus(
			recorder,
			httptest.NewRequest(http.MethodGet, "/api/v1/updates/missing", nil),
			"missing",
		)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404: %s", recorder.Code, recorder.Body)
		}
	})
}

// newOperationHandlers returns handlers backed by one live registry.
func newOperationHandlers(
	t *testing.T,
	sync func(context.Context) (*pkgsync.Result, error),
) (*Handlers, *operations.Registry) {
	t.Helper()
	registry := operations.NewRegistry()
	t.Cleanup(func() {
		if err := registry.Close(context.Background()); err != nil {
			t.Errorf("registry Close: %v", err)
		}
	})
	handlers := &Handlers{
		operations: registry,
		app: &testApplication{SyncFunc: func(
			ctx context.Context,
			_ ...pkgsync.Option,
		) (*pkgsync.Result, error) {
			return sync(ctx)
		}},
	}
	return handlers, registry
}

// postUpdate accepts one asynchronous update and returns its status.
func postUpdate(t *testing.T, handlers *Handlers) operations.Status {
	t.Helper()
	recorder := httptest.NewRecorder()
	handlers.HandleUpdate(
		recorder,
		httptest.NewRequest(http.MethodPost, "/api/v1/update", nil),
	)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", recorder.Code, recorder.Body)
	}
	status := decodeOperation(t, recorder.Body.Bytes())
	want := "/api/v1/updates/" + status.ID
	if location := recorder.Header().Get("Location"); location != want {
		t.Fatalf("Location = %q, want %q", location, want)
	}
	return status
}

// readOperation reads one operation status through the handler.
func readOperation(
	t *testing.T,
	handlers *Handlers,
	id string,
	wantCode int,
) operations.Status {
	t.Helper()
	recorder := httptest.NewRecorder()
	handlers.HandleOperationStatus(
		recorder,
		httptest.NewRequest(http.MethodGet, "/api/v1/updates/"+id, nil),
		id,
	)
	if recorder.Code != wantCode {
		t.Fatalf("status read = %d, want %d: %s", recorder.Code, wantCode, recorder.Body)
	}
	return decodeOperation(t, recorder.Body.Bytes())
}

// decodeOperation reads the operation status out of one response envelope.
func decodeOperation(t *testing.T, body []byte) operations.Status {
	t.Helper()
	var envelope struct {
		Data operations.Status `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("Unmarshal response: %v", err)
	}
	return envelope.Data
}
