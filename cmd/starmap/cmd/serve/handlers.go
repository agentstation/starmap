package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/agentstation/starmap/cmd/application"
	"github.com/agentstation/starmap/internal/cmd/provider"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/logging"
)

// APIHandlers holds the catalog and provides HTTP handlers for REST endpoints.
type APIHandlers struct {
	catalog catalogs.Catalog
}

// NewAPIHandlers creates a new API handlers instance using app context.
func NewAPIHandlers(app application.Application) (*APIHandlers, error) {
	cat, err := app.Catalog()
	if err != nil {
		return nil, fmt.Errorf("loading catalog: %w", err)
	}

	return &APIHandlers{
		catalog: cat,
	}, nil
}

// ModelsHandler handles /api/v1/models requests.
func (h *APIHandlers) ModelsHandler(w http.ResponseWriter, r *http.Request) {
	logging.Debug().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("Handling models request")

	switch r.Method {
	case http.MethodGet:
		h.handleGetModels(w, r)
	case http.MethodPost:
		h.handleSearchModels(w, r)
	default:
		h.methodNotAllowed(w, r)
	}
}

// ProvidersHandler handles /api/v1/providers requests.
func (h *APIHandlers) ProvidersHandler(w http.ResponseWriter, r *http.Request) {
	logging.Debug().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Msg("Handling providers request")

	switch r.Method {
	case http.MethodGet:
		h.handleGetProviders(w, r)
	default:
		h.methodNotAllowed(w, r)
	}
}

// ModelByIDHandler handles /api/v1/models/{id} requests.
func (h *APIHandlers) ModelByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.methodNotAllowed(w, r)
		return
	}

	// Extract model ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/models/")
	modelID := strings.Split(path, "/")[0]

	if modelID == "" {
		h.badRequest(w, "Model ID is required")
		return
	}

	logging.Debug().
		Str("model_id", modelID).
		Msg("Handling model by ID request")

	h.handleGetModelByID(w, r, modelID)
}

// ProviderByIDHandler handles /api/v1/providers/{id} requests.
func (h *APIHandlers) ProviderByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.methodNotAllowed(w, r)
		return
	}

	// Extract provider ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/providers/")
	parts := strings.Split(path, "/")
	providerID := parts[0]

	if providerID == "" {
		h.badRequest(w, "Provider ID is required")
		return
	}

	logging.Debug().
		Str("provider_id", providerID).
		Msg("Handling provider by ID request")

	// Check if this is a sub-resource request (e.g., /providers/{id}/models)
	if len(parts) > 1 && parts[1] == "models" {
		h.handleGetProviderModels(w, r, providerID)
		return
	}

	h.handleGetProviderByID(w, r, providerID)
}

// handleGetModels returns a list of all models.
func (h *APIHandlers) handleGetModels(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query()
	providerFilter := query.Get("provider")
	limitStr := query.Get("limit")
	offsetStr := query.Get("offset")

	// Parse limit and offset
	limit := 100 // Default limit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Get all models
	allModels := h.catalog.Models().List()

	// Apply provider filter if specified
	var filteredModels []*catalogs.Model
	if providerFilter != "" {
		// Get models from specific provider
		providers := h.catalog.Providers().List()
		for _, prov := range providers {
			if string(prov.ID) == providerFilter {
				for _, model := range prov.Models {
					filteredModels = append(filteredModels, model)
				}
				break
			}
		}
	} else {
		// Convert to pointer slice for compatibility
		filteredModels = make([]*catalogs.Model, len(allModels))
		for i := range allModels {
			filteredModels[i] = &allModels[i]
		}
	}

	// Apply pagination
	total := len(filteredModels)
	start := offset
	end := offset + limit

	if start >= total {
		filteredModels = []*catalogs.Model{}
	} else {
		if end > total {
			end = total
		}
		filteredModels = filteredModels[start:end]
	}

	// Create response
	response := map[string]any{
		"models": filteredModels,
		"pagination": map[string]any{
			"total":  total,
			"limit":  limit,
			"offset": offset,
			"count":  len(filteredModels),
		},
	}

	h.jsonResponse(w, http.StatusOK, response)
}

// handleGetModelByID returns a specific model by ID.
func (h *APIHandlers) handleGetModelByID(w http.ResponseWriter, _ *http.Request, modelID string) {
	// Use the catalog's FindModel method
	model, err := h.catalog.FindModel(modelID)
	if err != nil {
		if _, ok := err.(*errors.NotFoundError); ok {
			h.notFound(w, fmt.Sprintf("Model '%s' not found", modelID))
			return
		}
		h.internalError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, model)
}

// handleSearchModels handles POST /api/v1/models/search.
func (h *APIHandlers) handleSearchModels(w http.ResponseWriter, r *http.Request) {
	var searchReq struct {
		Query      string   `json:"query"`
		Providers  []string `json:"providers,omitempty"`
		Capability string   `json:"capability,omitempty"`
		MinContext int64    `json:"min_context,omitempty"`
		MaxPrice   float64  `json:"max_price,omitempty"`
		Limit      int      `json:"limit,omitempty"`
		Offset     int      `json:"offset,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&searchReq); err != nil {
		h.badRequest(w, "Invalid JSON request body")
		return
	}

	// Set default limit
	if searchReq.Limit == 0 {
		searchReq.Limit = 100
	}

	// Get all models and filter based on search criteria
	allModels := h.catalog.Models().List()
	results := make([]catalogs.Model, 0, len(allModels))

	for _, model := range allModels {
		// Apply filters
		if searchReq.Query != "" {
			queryLower := strings.ToLower(searchReq.Query)
			if !strings.Contains(strings.ToLower(model.Name), queryLower) &&
				!strings.Contains(strings.ToLower(model.ID), queryLower) &&
				!strings.Contains(strings.ToLower(model.Description), queryLower) {
				continue
			}
		}

		if len(searchReq.Providers) > 0 {
			// Check if model belongs to any of the requested providers
			found := false
			providers := h.catalog.Providers().List()
			for _, prov := range providers {
				for _, reqProv := range searchReq.Providers {
					if string(prov.ID) == reqProv {
						if _, exists := prov.Models[model.ID]; exists {
							found = true
							break
						}
					}
				}
				if found {
					break
				}
			}
			if !found {
				continue
			}
		}

		if searchReq.MinContext > 0 && model.Limits != nil {
			if model.Limits.ContextWindow < searchReq.MinContext {
				continue
			}
		}

		if searchReq.MaxPrice > 0 && model.Pricing != nil && model.Pricing.Tokens != nil && model.Pricing.Tokens.Input != nil {
			if model.Pricing.Tokens.Input.Per1M > searchReq.MaxPrice {
				continue
			}
		}

		results = append(results, model)
	}

	// Apply pagination
	total := len(results)
	start := searchReq.Offset
	end := searchReq.Offset + searchReq.Limit

	if start >= total {
		results = []catalogs.Model{}
	} else {
		if end > total {
			end = total
		}
		results = results[start:end]
	}

	// Create response
	response := map[string]any{
		"models": results,
		"search": map[string]any{
			"query":      searchReq.Query,
			"providers":  searchReq.Providers,
			"capability": searchReq.Capability,
		},
		"pagination": map[string]any{
			"total":  total,
			"limit":  searchReq.Limit,
			"offset": searchReq.Offset,
			"count":  len(results),
		},
	}

	h.jsonResponse(w, http.StatusOK, response)
}

// handleGetProviders returns a list of all providers.
func (h *APIHandlers) handleGetProviders(w http.ResponseWriter, _ *http.Request) {
	providers := h.catalog.Providers().List()

	// Create simplified provider list
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

		if prov.Catalog != nil && prov.Catalog.Docs != nil {
			providerInfo["docs_url"] = *prov.Catalog.Docs
		}

		providerList = append(providerList, providerInfo)
	}

	h.jsonResponse(w, http.StatusOK, map[string]any{
		"providers": providerList,
		"count":     len(providerList),
	})
}

// handleGetProviderByID returns a specific provider by ID.
func (h *APIHandlers) handleGetProviderByID(w http.ResponseWriter, _ *http.Request, providerID string) {
	prov, err := provider.Get(h.catalog, providerID)
	if err != nil {
		if _, ok := err.(*errors.NotFoundError); ok {
			h.notFound(w, fmt.Sprintf("Provider '%s' not found", providerID))
			return
		}
		h.internalError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, prov)
}

// handleGetProviderModels returns models for a specific provider.
func (h *APIHandlers) handleGetProviderModels(w http.ResponseWriter, _ *http.Request, providerID string) {
	prov, err := provider.Get(h.catalog, providerID)
	if err != nil {
		if _, ok := err.(*errors.NotFoundError); ok {
			h.notFound(w, fmt.Sprintf("Provider '%s' not found", providerID))
			return
		}
		h.internalError(w, err)
		return
	}

	// Convert map to slice
	models := make([]*catalogs.Model, 0, len(prov.Models))
	for _, model := range prov.Models {
		models = append(models, model)
	}

	response := map[string]any{
		"provider": map[string]any{
			"id":   prov.ID,
			"name": prov.Name,
		},
		"models": models,
		"count":  len(models),
	}

	h.jsonResponse(w, http.StatusOK, response)
}

// Helper methods for HTTP responses

func (h *APIHandlers) jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		logging.Error().Err(err).Msg("Failed to encode JSON response")
	}
}

func (h *APIHandlers) errorResponse(w http.ResponseWriter, status int, message string) {
	h.jsonResponse(w, status, map[string]string{
		"error":   http.StatusText(status),
		"message": message,
	})
}

func (h *APIHandlers) badRequest(w http.ResponseWriter, message string) {
	h.errorResponse(w, http.StatusBadRequest, message)
}

func (h *APIHandlers) notFound(w http.ResponseWriter, message string) {
	h.errorResponse(w, http.StatusNotFound, message)
}

func (h *APIHandlers) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	h.errorResponse(w, http.StatusMethodNotAllowed, fmt.Sprintf("Method %s not allowed", r.Method))
}

func (h *APIHandlers) internalError(w http.ResponseWriter, err error) {
	logging.Error().Err(err).Msg("Internal server error")
	h.errorResponse(w, http.StatusInternalServerError, "Internal server error")
}
