package handlers

import (
	"net/http"
	"runtime"
	"time"

	"github.com/agentstation/starmap/internal/server/events"
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
	h.broker.Publish(events.SyncCompleted, map[string]any{
		"total_changes":     result.TotalChanges,
		"providers_changed": result.ProvidersChanged,
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
// @Description Get comprehensive server and catalog statistics
// @Tags admin
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=object}
// @Failure 500 {object} response.Response{error=response.Error}
// @Security ApiKeyAuth
// @Router /api/v1/stats [get].
func (h *Handlers) HandleStats(w http.ResponseWriter, r *http.Request) {
	cat, err := h.app.Catalog()
	if err != nil {
		response.InternalError(w, err)
		return
	}

	models := cat.Models().List()
	providers := cat.Providers().List()

	// Get runtime stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Get server from context (if available) for uptime
	uptime := time.Duration(0)
	if srv, ok := r.Context().Value("server").(interface{ StartTime() time.Time }); ok {
		uptime = time.Since(srv.StartTime())
	}

	response.OK(w, map[string]any{
		"runtime": map[string]any{
			"uptime_seconds": int64(uptime.Seconds()),
			"goroutines":     runtime.NumGoroutine(),
			"memory_mb":      memStats.Alloc / 1024 / 1024,
			"memory_sys_mb":  memStats.Sys / 1024 / 1024,
		},
		"catalog": map[string]any{
			"models_total":    len(models),
			"providers_total": len(providers),
		},
		"events": map[string]any{
			"published_total": h.broker.EventsPublished(),
			"dropped_total":   h.broker.EventsDropped(),
			"queue_depth":     h.broker.QueueDepth(),
		},
		"realtime": map[string]any{
			"websocket_clients": h.wsHub.ClientCount(),
			"sse_clients":       h.sseBroadcaster.ClientCount(),
		},
		"cache": h.cache.GetStats(),
	})
}
