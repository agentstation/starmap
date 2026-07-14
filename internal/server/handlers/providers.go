package handlers

import (
	"net/http"

	"github.com/agentstation/starmap/internal/catalog/query"
	"github.com/agentstation/starmap/internal/cli/provider"
	"github.com/agentstation/starmap/internal/server/response"
)

// HandleListProviders handles GET /api/v1/providers.
// @Summary List providers
// @Description List all providers
// @Tags providers
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=object}
// @Failure 500 {object} response.Response{error=response.Error}
// @Security ApiKeyAuth
// @Router /api/v1/providers [get].
func (h *Handlers) HandleListProviders(w http.ResponseWriter, _ *http.Request) {
	state, err := h.app.CatalogState()
	if err != nil {
		response.InternalError(w, err)
		return
	}
	w.Header().Set("X-Starmap-Generation-ID", state.GenerationID)
	// Check cache
	if cached, found := h.cache.GetGeneration(state.Sequence, state.GenerationID, "providers"); found {
		response.OK(w, cached)
		return
	}

	// Get catalog
	cat := state.Catalog

	providers := query.Providers(cat.Providers().List(), query.ProviderOptions{})

	// Build simplified provider list
	providerList := make([]map[string]any, 0, len(providers))
	for _, prov := range providers {
		providerInfo := map[string]any{
			"id":          prov.ID,
			"name":        prov.Name,
			"model_count": len(prov.Models),
		}

		if prov.Headquarters != nil {
			providerInfo["headquarters"] = *prov.Headquarters
		}

		if prov.Catalog != nil && len(prov.Catalog.Sources) > 0 && prov.Catalog.Sources[0].Docs != "" {
			providerInfo["docs_url"] = prov.Catalog.Sources[0].Docs
		}

		providerList = append(providerList, providerInfo)
	}

	result := map[string]any{
		"providers":        providerList,
		responseFieldCount: len(providerList),
	}

	// Cache result
	h.cache.SetGeneration(state.Sequence, state.GenerationID, "providers", result)

	response.OK(w, result)
}

// HandleGetProvider handles GET /api/v1/providers/{id}.
// @Summary Get provider by ID
// @Description Retrieve detailed information about a specific provider
// @Tags providers
// @Accept json
// @Produce json
// @Param id path string true "Provider ID"
// @Success 200 {object} response.Response{data=catalogs.Provider}
// @Failure 404 {object} response.Response{error=response.Error}
// @Failure 500 {object} response.Response{error=response.Error}
// @Security ApiKeyAuth
// @Router /api/v1/providers/{id} [get].
func (h *Handlers) HandleGetProvider(w http.ResponseWriter, _ *http.Request, providerID string) {
	state, err := h.app.CatalogState()
	if err != nil {
		response.InternalError(w, err)
		return
	}
	w.Header().Set("X-Starmap-Generation-ID", state.GenerationID)
	// Check cache
	cacheKey := "provider:" + providerID
	if cached, found := h.cache.GetGeneration(state.Sequence, state.GenerationID, cacheKey); found {
		response.OK(w, cached)
		return
	}

	// Get catalog
	cat := state.Catalog

	// Get provider
	prov, err := provider.Get(cat, providerID)
	if err != nil {
		response.ErrorFromType(w, err)
		return
	}

	// Cache result
	h.cache.SetGeneration(state.Sequence, state.GenerationID, cacheKey, prov)

	response.OK(w, prov)
}

// HandleGetProviderModels handles GET /api/v1/providers/{id}/models.
// @Summary Get provider models
// @Description List all models for a specific provider
// @Tags providers
// @Accept json
// @Produce json
// @Param id path string true "Provider ID"
// @Success 200 {object} response.Response{data=object}
// @Failure 404 {object} response.Response{error=response.Error}
// @Failure 500 {object} response.Response{error=response.Error}
// @Security ApiKeyAuth
// @Router /api/v1/providers/{id}/models [get].
func (h *Handlers) HandleGetProviderModels(w http.ResponseWriter, _ *http.Request, providerID string) {
	// Get catalog
	cat, err := h.app.Catalog()
	if err != nil {
		response.InternalError(w, err)
		return
	}

	// Get provider
	prov, err := provider.Get(cat, providerID)
	if err != nil {
		response.ErrorFromType(w, err)
		return
	}

	models, err := query.CatalogModels(cat, string(prov.ID))
	if err != nil {
		response.ErrorFromType(w, err)
		return
	}

	result := map[string]any{
		"provider": map[string]any{
			"id":   prov.ID,
			"name": prov.Name,
		},
		responseFieldModels: models,
		responseFieldCount:  len(models),
	}

	response.OK(w, result)
}
