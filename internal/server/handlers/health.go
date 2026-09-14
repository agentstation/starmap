package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/agentstation/starmap/internal/server/response"
	"github.com/agentstation/starmap/runtime/status"
)

// HandleHealth handles GET /api/v1/health.
// @Summary Health check
// @Description Health check endpoint (liveness probe)
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=object}
// @Router /api/v1/health [get].
func (h *Handlers) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	response.OK(w, map[string]any{
		"status":  "healthy",
		"service": "starmap-api",
		"version": "v1",
	})
}

// HandleReady handles GET /api/v1/ready.
// @Summary Readiness check
// @Description Readiness check including cache and data source status
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=object}
// @Failure 503 {object} response.Response{error=response.Error}
// @Router /api/v1/ready [get].
func (h *Handlers) HandleReady(w http.ResponseWriter, _ *http.Request) {
	readiness, err := h.app.Readiness()
	if err != nil {
		response.ServiceUnavailable(w, "Catalog not available")
		return
	}
	if !readiness.Ready {
		reasons := make([]string, 0, len(readiness.Issues))
		for _, issue := range readiness.Issues {
			reasons = append(reasons, issue.Code+": "+issue.Message)
		}
		response.ServiceUnavailable(w, strings.Join(reasons, "; "))
		return
	}
	runtimeStatus := h.app.RuntimeStatus()
	if runtimeStatus.AuthorityRequired && !runtimeStatus.Usable {
		body := response.Fail("catalog_authority_unavailable", "Catalog authority does not permit new work", "")
		body.Data = map[string]any{"status": "not_ready", "catalog": readiness, "runtime": runtimeReadiness(runtimeStatus)}
		response.JSON(w, http.StatusServiceUnavailable, body)
		return
	}

	response.OK(w, map[string]any{
		"status":  "ready",
		"catalog": readiness,
		"runtime": runtimeReadiness(runtimeStatus),
		"cache": map[string]any{
			"items": h.cache.ItemCount(),
		},
		"sse_clients": h.sseBroadcaster.ClientCount(),
	})
}

// runtimeReadiness reports the connected runtime state. Every value is a
// bounded code or an age, so the readiness body carries no message text.
func runtimeReadiness(status status.Status) map[string]any {
	return map[string]any{
		"usable":                       status.Usable,
		"retention":                    retentionReadiness(status.Retention),
		"catalog_available":            status.CatalogAvailable,
		"authority_required":           status.AuthorityRequired,
		"authority_ready":              status.AuthorityReady,
		"permission_valid":             status.PermissionValid,
		"required_permission_revision": status.RequiredPermissionRevision,
		"enforced_permission_revision": status.EnforcedPermissionRevision,
		"permission_valid_until":       status.PermissionValidUntil,
		"generation_id":                status.GenerationID,
		"source_kind":                  string(status.SourceKind),
		"source_health":                string(status.SourceHealth),
		"source_reason":                status.SourceReason,
		"upstream_health":              string(status.UpstreamHealth),
		"acquisition_health":           string(status.AcquisitionHealth),
		"accepted_acquisition_sources": status.AcceptedAcquisitionSources,
		"source_configuration":         status.SourceConfiguration,
		"source_activities":            status.SourceActivities,
		"freshness":                    string(status.Freshness),
		"catalog_age_seconds":          int64(status.CatalogAge / time.Second),
		"channel_freshness":            string(status.ChannelFreshness),
		"channel_age_seconds":          int64(status.ChannelAge / time.Second),
		"source_check_freshness":       string(status.SourceCheckFreshness),
		"instance_identity":            status.InstanceIdentity,
		"chain_hops":                   len(status.Chain),
		"fallback":                     status.Fallback,
		"fallback_reason":              status.FallbackReason,
		"lease":                        status.Lease,
	}
}

// retentionReadiness exposes bounded maintenance diagnostics without storage reads or raw errors.
func retentionReadiness(retention status.RetentionStatus) map[string]any {
	return map[string]any{
		"enabled": retention.Enabled, "interval_seconds": int64(retention.Interval / time.Second),
		"max_generations": retention.MaxGenerations, "max_bytes": retention.MaxBytes,
		"scan_entries": retention.ScanEntries, "input_max_bytes": retention.InputMaxBytes,
		"attempted_at": retention.AttemptedAt, "succeeded_at": retention.SucceededAt,
		"health": string(retention.Health), "reason": retention.Reason, "generation_collection": retention.GenerationCollection,
		"generations": retention.Generations, "generation_bytes": retention.GenerationBytes,
		"protected_generations": retention.ProtectedGenerations, "protected_bytes": retention.ProtectedBytes,
		"removed_generations": retention.RemovedGenerations, "scanned_inputs": retention.ScannedInputs,
		"input_bytes": retention.InputBytes, "removed_inputs": retention.RemovedInputs, "over_limit": retention.OverLimit,
	}
}
