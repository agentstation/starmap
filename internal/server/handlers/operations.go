package handlers

import (
	"net/http"

	"github.com/agentstation/starmap/internal/server/response"
)

// HandleOperations handles GET /api/v1/operations.
// @Summary Catalog operational state
// @Description Get current generation, source freshness, last synchronization, degraded sources, and scheduler state
// @Tags admin
// @Produce json
// @Success 200 {object} response.Response{data=object}
// @Failure 500 {object} response.Response{error=response.Error}
// @Security ApiKeyAuth
// @Router /api/v1/operations [get].
func (h *Handlers) HandleOperations(w http.ResponseWriter, r *http.Request) {
	state, err := h.app.OperationalState(r.Context())
	if err != nil {
		response.InternalError(w, err)
		return
	}
	response.OK(w, state)
}
