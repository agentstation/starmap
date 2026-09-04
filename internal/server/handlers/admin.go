package handlers

import (
	"context"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/server/operations"
	"github.com/agentstation/starmap/internal/server/response"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/pkg/sync"
)

// HandleUpdate handles POST /api/v1/update.
// @Summary Trigger catalog update
// @Description Accept an asynchronous catalog synchronization
// @Tags admin
// @Accept json
// @Produce json
// @Param provider query string false "Update specific provider only"
// @Param source query string false "Update one source only (local_catalog, providers, models_dev_http, or models_dev_git)"
// @Success 202 {object} response.Response{data=operations.Status}
// @Failure 500 {object} response.Response{error=response.Error}
// @Security ApiKeyAuth
// @Router /api/v1/update [post].
func (h *Handlers) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	if h.operations == nil {
		response.ServiceUnavailable(w, "asynchronous operations are unavailable")
		return
	}

	// Read the filters before the response returns. The operation then owns no
	// reference to the request, which ends long before the run does.
	var opts []sync.Option
	if provider := r.URL.Query().Get("provider"); provider != "" {
		opts = append(opts, sync.WithProvider(catalogs.ProviderID(provider)))
	}
	if source := r.URL.Query().Get("source"); source != "" {
		opts = append(opts, sync.WithSources(sources.ID(source)))
	}

	status, err := h.operations.Start(
		operations.KindCatalogUpdate,
		func(ctx context.Context) (map[string]any, error) {
			return h.runCatalogUpdate(ctx, opts)
		},
	)
	if err != nil {
		response.ErrorFromType(w, h.logger, err)
		return
	}
	h.log().Info().
		Str("operation_id", status.ID).
		Str("operation_kind", status.Kind.String()).
		Msg("Catalog update accepted")
	w.Header().Set("Location", operationLocation(r, status.ID))
	response.Accepted(w, status)
}

// runCatalogUpdate runs one accepted catalog update. It returns a bounded
// detail summary, so the status carries no provider message text.
func (h *Handlers) runCatalogUpdate(
	ctx context.Context,
	opts []sync.Option,
) (map[string]any, error) {
	result, err := h.app.Sync(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"total_changes":     result.TotalChanges,
		"providers_changed": result.ProvidersChanged,
		"dry_run":           result.DryRun,
		"generation_id":     result.GenerationID,
		"sync_run_id":       result.SyncRunID,
	}, nil
}

// HandleOperationStatus handles GET /api/v1/updates/{id}.
// @Summary Read an update operation
// @Description Report whether one update runs, finished, or stopped
// @Tags admin
// @Produce json
// @Param id path string true "Operation identity"
// @Success 200 {object} response.Response{data=operations.Status}
// @Failure 404 {object} response.Response{error=response.Error}
// @Security ApiKeyAuth
// @Router /api/v1/updates/{id} [get].
func (h *Handlers) HandleOperationStatus(
	w http.ResponseWriter,
	_ *http.Request,
	id string,
) {
	status, found := h.lookupOperation(w, id)
	if !found {
		return
	}
	response.OK(w, status)
}

// HandleOperationCancel handles DELETE /api/v1/updates/{id}.
// @Summary Cancel an update operation
// @Description Ask one accepted or running update to stop
// @Tags admin
// @Produce json
// @Param id path string true "Operation identity"
// @Success 200 {object} response.Response{data=operations.Status}
// @Failure 404 {object} response.Response{error=response.Error}
// @Security ApiKeyAuth
// @Router /api/v1/updates/{id} [delete].
func (h *Handlers) HandleOperationCancel(
	w http.ResponseWriter,
	_ *http.Request,
	id string,
) {
	if h.operations == nil {
		response.ServiceUnavailable(w, "asynchronous operations are unavailable")
		return
	}
	status, found := h.operations.Cancel(id)
	if !found {
		response.NotFound(w, "Operation not found", id)
		return
	}
	h.log().Info().
		Str("operation_id", status.ID).
		Str("operation_state", status.State.String()).
		Msg("Catalog update cancellation requested")
	response.OK(w, status)
}

// lookupOperation reads one operation status and writes the failure response.
func (h *Handlers) lookupOperation(
	w http.ResponseWriter,
	id string,
) (operations.Status, bool) {
	if h.operations == nil {
		response.ServiceUnavailable(w, "asynchronous operations are unavailable")
		return operations.Status{}, false
	}
	status, found := h.operations.Status(id)
	if !found {
		response.NotFound(w, "Operation not found", id)
		return operations.Status{}, false
	}
	return status, true
}

// operationLocation names the status route of one accepted operation. It reuses
// the request path, so a server with a custom path prefix stays correct.
func operationLocation(r *http.Request, id string) string {
	return strings.TrimSuffix(r.URL.Path, "/update") + "/updates/" + id
}

// HandleStats handles GET /api/v1/stats.
// @Summary Catalog statistics
// @Description Get complete server and catalog statistics
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
		response.InternalError(w, h.logger, err)
		return
	}

	models := cat.Definitions()
	providers := cat.Providers().List()
	state, err := h.app.CatalogState()
	if err != nil {
		response.InternalError(w, h.logger, err)
		return
	}
	sm, err := h.app.Starmap()
	if err != nil {
		response.InternalError(w, h.logger, err)
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
