package handlers

import (
	"net/http"
	"time"

	"github.com/agentstation/starmap/internal/server/response"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sync"
)

// HandleUpdate handles POST /api/v1/update.
// @Summary Trigger catalog update
// @Description Manually trigger catalog synchronization
// @Tags admin
// @Accept json
// @Produce json
// @Param provider query string false "Update specific provider only"
// @Success 200 {object} response.Response{data=object}
// @Failure 500 {object} response.Response{error=response.Error}
// @Security ApiKeyAuth
// @Router /api/v1/update [post].
func (h *Handlers) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	providerFilter := r.URL.Query().Get("provider")

	sm, err := h.app.Starmap()
	if err != nil {
		response.InternalError(w, err)
		return
	}

	// Build sync options
	var opts []sync.Option
	if providerFilter != "" {
		opts = append(opts, sync.WithProvider(catalogs.ProviderID(providerFilter)))
	}

	// Run sync
	result, err := sm.Sync(r.Context(), opts...)
	if err != nil {
		response.InternalError(w, err)
		return
	}

	// Invalidate cache
	h.cache.Clear()

	// Broadcast update event
	h.broadcastFn("sync.completed", map[string]any{
		"total_changes":     result.TotalChanges,
		"providers_changed": result.ProvidersChanged,
		"timestamp":         time.Now(),
	})

	response.OK(w, map[string]any{
		"status":            "completed",
		"total_changes":     result.TotalChanges,
		"providers_changed": result.ProvidersChanged,
		"dry_run":           result.DryRun,
	})
}

// HandleStats handles GET /api/v1/stats.
// @Summary Catalog statistics
// @Description Get catalog statistics (model count, provider count, last sync)
// @Tags admin
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=object}
// @Failure 500 {object} response.Response{error=response.Error}
// @Security ApiKeyAuth
// @Router /api/v1/stats [get].
func (h *Handlers) HandleStats(w http.ResponseWriter, _ *http.Request) {
	cat, err := h.app.Catalog()
	if err != nil {
		response.InternalError(w, err)
		return
	}

	models := cat.Models().List()
	providers := cat.Providers().List()

	response.OK(w, map[string]any{
		"models": map[string]any{
			"total": len(models),
		},
		"providers": map[string]any{
			"total": len(providers),
		},
		"cache": h.cache.GetStats(),
		"realtime": map[string]any{
			"websocket_clients": h.wsHub.ClientCount(),
			"sse_clients":       h.sseBroadcaster.ClientCount(),
		},
	})
}
