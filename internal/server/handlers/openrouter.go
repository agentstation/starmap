package handlers

import (
	"net/http"

	"github.com/agentstation/starmap/internal/server/openrouter"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// HandleOpenRouterModel handles GET /api/v1/model/{author}/{slug}.
// @Summary Get an OpenRouter-compatible model by author and slug
// @Description Resolve a canonical model, known alias, or configured variant
// @Tags openrouter
// @Produce json
// @Param author path string true "Canonical author ID or alias"
// @Param slug path string true "Model slug or configured variant"
// @Success 200 {object} openrouter.ModelEnvelope
// @Failure 401 {object} openrouter.ErrorEnvelope
// @Failure 404 {object} openrouter.ErrorEnvelope
// @Failure 500 {object} openrouter.ErrorEnvelope
// @Security ApiKeyAuth
// @Router /api/v1/model/{author}/{slug} [get].
func (h *Handlers) HandleOpenRouterModel(
	w http.ResponseWriter,
	_ *http.Request,
	authorID string,
	slug string,
	pathPrefix string,
) {
	state, err := h.app.CatalogState()
	if err != nil || state.Catalog == nil {
		h.logOpenRouterError(err)
		openrouter.WriteError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}
	w.Header().Set("X-Starmap-Generation-ID", state.GenerationID)
	model, err := openrouter.ProjectModel(
		state.Catalog,
		catalogs.AuthorID(authorID),
		slug,
		pathPrefix,
	)
	if err != nil {
		h.logOpenRouterError(err)
		openrouter.WriteProjectionError(w, err)
		return
	}
	openrouter.WriteModel(w, model)
}

// HandleOpenRouterEndpoints handles
// GET /api/v1/models/{author}/{slug}/endpoints.
// @Summary List OpenRouter-compatible provider endpoints for a model
// @Description Project every eligible exact provider offering for one model
// @Tags openrouter
// @Produce json
// @Param author path string true "Canonical author ID or alias"
// @Param slug path string true "Model slug or configured variant"
// @Success 200 {object} openrouter.EndpointsEnvelope
// @Failure 401 {object} openrouter.ErrorEnvelope
// @Failure 404 {object} openrouter.ErrorEnvelope
// @Failure 500 {object} openrouter.ErrorEnvelope
// @Security ApiKeyAuth
// @Router /api/v1/models/{author}/{slug}/endpoints [get].
func (h *Handlers) HandleOpenRouterEndpoints(
	w http.ResponseWriter,
	_ *http.Request,
	authorID string,
	slug string,
) {
	state, err := h.app.CatalogState()
	if err != nil || state.Catalog == nil {
		h.logOpenRouterError(err)
		openrouter.WriteError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}
	w.Header().Set("X-Starmap-Generation-ID", state.GenerationID)
	endpoints, err := openrouter.ProjectEndpoints(
		state.Catalog,
		catalogs.AuthorID(authorID),
		slug,
	)
	if err != nil {
		h.logOpenRouterError(err)
		openrouter.WriteProjectionError(w, err)
		return
	}
	openrouter.WriteEndpoints(w, endpoints)
}

func (h *Handlers) logOpenRouterError(err error) {
	if err == nil || h.logger == nil {
		return
	}
	h.logger.Debug().Err(err).Msg("OpenRouter catalog projection failed")
}
