package handlers

import (
	"net/http"
	"runtime"
	"time"

	"github.com/agentstation/starmap/internal/server/response"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/pkg/sync"
)

// HandleUpdate handles POST /api/v1/update.
// @Summary Trigger catalog update
// @Description Manually trigger catalog synchronization
// @Tags admin
// @Accept json
// @Produce json
// @Param provider query string false "Update specific provider only"
// @Param source query string false "Update one source only (local_catalog, providers, models_dev_http, or models_dev_git)"
// @Success 200 {object} response.Response{data=object}
// @Failure 500 {object} response.Response{error=response.Error}
// @Security ApiKeyAuth
// @Router /api/v1/update [post].
func (h *Handlers) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	providerFilter := r.URL.Query().Get("provider")
	sourceFilter := r.URL.Query().Get("source")

	// Build sync options
	var opts []sync.Option
	if providerFilter != "" {
		opts = append(opts, sync.WithProvider(catalogs.ProviderID(providerFilter)))
	}
	if sourceFilter != "" {
		opts = append(opts, sync.WithSources(sources.ID(sourceFilter)))
	}

	// Run explicitly composed acquisition.
	result, err := h.app.Sync(r.Context(), opts...)
	if err != nil {
		response.InternalError(w, err)
		return
	}

	response.OK(w, map[string]any{
		"status":            "completed",
		"total_changes":     result.TotalChanges,
		"providers_changed": result.ProvidersChanged,
		"dry_run":           result.DryRun,
		"generation_id":     result.GenerationID,
		"sync_run_id":       result.SyncRunID,
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
func (h *Handlers) HandleStats(w http.ResponseWriter, _ *http.Request) {
	cat, err := h.app.Catalog()
	if err != nil {
		response.InternalError(w, err)
		return
	}

	models := cat.Definitions()
	providers := cat.Providers().List()
	state, err := h.app.CatalogState()
	if err != nil {
		response.InternalError(w, err)
		return
	}
	sm, err := h.app.Starmap()
	if err != nil {
		response.InternalError(w, err)
		return
	}

	// Get runtime stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	uptime := time.Duration(0)
	if !h.startTime.IsZero() {
		uptime = time.Since(h.startTime)
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
			"generation_id":   state.GenerationID,
			"generated_at":    state.GeneratedAt,
			"age_seconds":     catalogAgeSeconds(state.GeneratedAt),
		},
		"realtime": map[string]any{
			"sse":         h.sseBroadcaster.Health(),
			"publication": sm.HookStats(),
		},
		"cache": h.cache.GetStats(),
	})
}

func catalogAgeSeconds(generatedAt time.Time) int64 {
	if generatedAt.IsZero() {
		return 0
	}
	return int64(time.Since(generatedAt) / time.Second)
}
