package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/server/operations"
	"github.com/agentstation/starmap/server/administration"
)

func TestRetainedAdministrativeOperationKeepsStatusContract(t *testing.T) {
	for _, test := range []struct {
		name              string
		action            administration.Action
		complete, success bool
		want              operations.State
		auditState        administration.State
		code              int
	}{
		{"succeeded", administration.RefreshCatalog, true, true, operations.StateSucceeded, administration.Succeeded, http.StatusOK},
		{"failed", administration.RefreshCatalog, true, false, operations.StateFailed, administration.Failed, http.StatusOK},
		{"interrupted", administration.RefreshCatalog, false, false, operations.StateFailed, administration.Interrupted, http.StatusOK},
		{"different action", administration.CancelOperation, true, true, "", administration.Succeeded, http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := administration.Config{StateDirectory: filepath.Join(t.TempDir(), "state"), Audience: "test"}
			manager, token, err := administration.Initialize(t.Context(), cfg, "operator")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = manager.Close() })
			actor, ok := manager.Authenticate(token, "test")
			if !ok {
				t.Fatal("administrator rejected")
			}
			receipt, err := manager.StartOperation(t.Context(), actor, test.action, "catalog")
			if err != nil {
				t.Fatal(err)
			}
			if test.complete {
				if _, err := manager.Complete(t.Context(), receipt.ID, test.success); err != nil {
					t.Fatal(err)
				}
			}
			if err := manager.Close(); err != nil {
				t.Fatal(err)
			}
			restored, err := administration.Open(t.Context(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer restored.Close()
			client, err := starmap.New()
			if err != nil {
				t.Fatal(err)
			}
			config := DefaultConfig()
			config.RateLimit = 0
			srv, err := New(client, config, WithAdministration(restored, "test"), WithSyncer(&administrativeSyncer{}))
			if err != nil {
				t.Fatal(err)
			}
			defer srv.Shutdown(context.Background())
			request := httptest.NewRequest(http.MethodGet, "/api/v1/updates/"+receipt.ID, nil)
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			srv.Handler().ServeHTTP(response, request)
			if response.Code != test.code {
				t.Fatalf("status=%d want=%d", response.Code, test.code)
			}
			if test.code != http.StatusOK {
				return
			}
			var envelope struct {
				Data operations.Status `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			got := envelope.Data
			if got.ID != receipt.ID || got.Kind != operations.KindCatalogUpdate || got.State != test.want || !got.State.Terminal() ||
				!got.AcceptedAt.Equal(receipt.AcceptedAt) || got.CompletedAt.IsZero() {
				t.Fatalf("retained response violates operation status contract: %s", response.Body.String())
			}
			if !got.StartedAt.IsZero() {
				t.Fatal("retained receipt invented an execution start time")
			}
			audit, ok := got.Detail["retained_receipt"].(map[string]any)
			if !ok || audit["state"] != string(test.auditState) || audit["id"] != receipt.ID {
				t.Fatal("retained status omitted its durable evidence")
			}
		})
	}
}
