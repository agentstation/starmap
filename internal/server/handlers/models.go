package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/agentstation/starmap/internal/server/filter"
	"github.com/agentstation/starmap/internal/server/response"
	"github.com/agentstation/starmap/pkg/catalogs"
)

// HandleListModels handles GET /api/v1/models.
// @Summary List models
// @Description List all models with optional filtering
// @Tags models
// @Accept json
// @Produce json
// @Param id query string false "Filter by exact model ID"
// @Param name query string false "Filter by exact model name (case-insensitive)"
// @Param name_contains query string false "Filter by partial model name match"
// @Param provider query string false "Filter by provider ID"
// @Param modality_input query string false "Filter by input modality (comma-separated)"
// @Param modality_output query string false "Filter by output modality (comma-separated)"
// @Param feature query string false "Filter by feature (streaming, tool_calls, etc.)"
// @Param tag query string false "Filter by tag (comma-separated)"
// @Param open_weights query boolean false "Filter by open weights status"
// @Param min_context query integer false "Minimum context window size"
// @Param max_context query integer false "Maximum context window size"
// @Param sort query string false "Sort field (id, name, release_date, context_window, created_at, updated_at)"
// @Param order query string false "Sort order (asc, desc)"
// @Param limit query integer false "Maximum number of results (default: 100, max: 1000)"
// @Param offset query integer false "Result offset for pagination"
// @Success 200 {object} response.Response{data=object}
// @Failure 400 {object} response.Response{error=response.Error}
// @Failure 500 {object} response.Response{error=response.Error}
// @Security ApiKeyAuth
// @Router /api/v1/models [get].
func (h *Handlers) HandleListModels(w http.ResponseWriter, r *http.Request) {
	// Check cache
	cacheKey := "models:" + r.URL.RawQuery
	if cached, found := h.cache.Get(cacheKey); found {
		response.OK(w, cached)
		return
	}

	// Get catalog
	cat, err := h.app.Catalog()
	if err != nil {
		response.InternalError(w, err)
		return
	}

	// Parse filters
	f := filter.ParseModelFilter(r)

	// Get and filter models
	allModels := cat.Models().List()
	filtered := f.Apply(allModels)

	// Apply pagination
	total := len(filtered)
	start := f.Offset
	end := f.Offset + f.Limit

	if start >= total {
		filtered = []catalogs.Model{}
	} else {
		if end > total {
			end = total
		}
		filtered = filtered[start:end]
	}

	// Build response
	result := map[string]any{
		"models": filtered,
		"pagination": map[string]any{
			"total":  total,
			"limit":  f.Limit,
			"offset": f.Offset,
			"count":  len(filtered),
		},
	}

	// Cache result
	h.cache.Set(cacheKey, result)

	response.OK(w, result)
}

// HandleGetModel handles GET /api/v1/models/{id}.
// @Summary Get model by ID
// @Description Retrieve detailed information about a specific model
// @Tags models
// @Accept json
// @Produce json
// @Param id path string true "Model ID"
// @Success 200 {object} response.Response{data=catalogs.Model}
// @Failure 404 {object} response.Response{error=response.Error}
// @Failure 500 {object} response.Response{error=response.Error}
// @Security ApiKeyAuth
// @Router /api/v1/models/{id} [get].
func (h *Handlers) HandleGetModel(w http.ResponseWriter, _ *http.Request, modelID string) {
	// Check cache
	cacheKey := "model:" + modelID
	if cached, found := h.cache.Get(cacheKey); found {
		response.OK(w, cached)
		return
	}

	// Get catalog
	cat, err := h.app.Catalog()
	if err != nil {
		response.InternalError(w, err)
		return
	}

	// Find model
	model, err := cat.FindModel(modelID)
	if err != nil {
		response.ErrorFromType(w, err)
		return
	}

	// Cache result
	h.cache.Set(cacheKey, model)

	response.OK(w, model)
}

// SearchRequest represents the POST /api/v1/models/search request body.
type SearchRequest struct {
	IDs           []string          `json:"ids,omitempty"`
	NameContains  string            `json:"name_contains,omitempty"`
	Provider      string            `json:"provider,omitempty"`
	Modalities    *SearchModalities `json:"modalities,omitempty"`
	Features      map[string]bool   `json:"features,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	OpenWeights   *bool             `json:"open_weights,omitempty"`
	ContextWindow *IntRange         `json:"context_window,omitempty"`
	OutputTokens  *IntRange         `json:"output_tokens,omitempty"`
	ReleaseDate   *DateRange        `json:"release_date,omitempty"`
	Sort          string            `json:"sort,omitempty"`
	Order         string            `json:"order,omitempty"`
	MaxResults    int               `json:"max_results,omitempty"`
}

// SearchModalities specifies modality requirements.
type SearchModalities struct {
	Input  []string `json:"input,omitempty"`
	Output []string `json:"output,omitempty"`
}

// IntRange represents an integer range filter.
type IntRange struct {
	Min int64 `json:"min,omitempty"`
	Max int64 `json:"max,omitempty"`
}

// DateRange represents a date range filter.
type DateRange struct {
	After  string `json:"after,omitempty"`
	Before string `json:"before,omitempty"`
}

// HandleSearchModels handles POST /api/v1/models/search.
// @Summary Search models
// @Description Advanced search with multiple criteria
// @Tags models
// @Accept json
// @Produce json
// @Param search body SearchRequest true "Search criteria"
// @Success 200 {object} response.Response{data=object}
// @Failure 400 {object} response.Response{error=response.Error}
// @Failure 500 {object} response.Response{error=response.Error}
// @Security ApiKeyAuth
// @Router /api/v1/models/search [post].
func (h *Handlers) HandleSearchModels(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid JSON request body", err.Error())
		return
	}

	// Get catalog
	cat, err := h.app.Catalog()
	if err != nil {
		response.InternalError(w, err)
		return
	}

	// Convert search request to filter
	f := filter.ModelFilter{
		NameContains: req.NameContains,
		Provider:     req.Provider,
		Features:     req.Features,
		Tags:         req.Tags,
		OpenWeights:  req.OpenWeights,
		Sort:         req.Sort,
		Order:        req.Order,
		Limit:        100,
		MaxResults:   req.MaxResults,
	}

	if req.Modalities != nil {
		f.ModalityInput = req.Modalities.Input
		f.ModalityOutput = req.Modalities.Output
	}

	if req.ContextWindow != nil {
		f.MinContext = req.ContextWindow.Min
		f.MaxContext = req.ContextWindow.Max
	}

	if req.OutputTokens != nil {
		f.MinOutput = req.OutputTokens.Min
		f.MaxOutput = req.OutputTokens.Max
	}

	// Apply filters
	allModels := cat.Models().List()
	results := f.Apply(allModels)

	// Filter by IDs if specified
	if len(req.IDs) > 0 {
		filtered := make([]catalogs.Model, 0, len(req.IDs))
		idMap := make(map[string]bool)
		for _, id := range req.IDs {
			idMap[id] = true
		}
		for _, model := range results {
			if idMap[model.ID] {
				filtered = append(filtered, model)
			}
		}
		results = filtered
	}

	// Apply max results limit
	if req.MaxResults > 0 && len(results) > req.MaxResults {
		results = results[:req.MaxResults]
	}

	// Build response
	result := map[string]any{
		"models": results,
		"count":  len(results),
	}

	response.OK(w, result)
}
