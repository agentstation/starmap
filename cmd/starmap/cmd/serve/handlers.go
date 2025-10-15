package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/agentstation/starmap/cmd/application"
	"github.com/agentstation/starmap/internal/cmd/provider"
	"github.com/agentstation/starmap/internal/embedded/openapi"
	"github.com/agentstation/starmap/internal/server/cache"
	"github.com/agentstation/starmap/internal/server/filter"
	"github.com/agentstation/starmap/internal/server/response"
	"github.com/agentstation/starmap/internal/server/sse"
	ws "github.com/agentstation/starmap/internal/server/websocket"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/sync"
)

// APIServer holds the server state including catalog, cache, and real-time components.
type APIServer struct {
	app         application.Application
	cache       *cache.Cache
	wsHub       *ws.Hub
	sseBroadcaster *sse.Broadcaster
	upgrader    websocket.Upgrader
}

// NewAPIServer creates a new API server instance.
func NewAPIServer(app application.Application) (*APIServer, error) {
	logger := app.Logger()

	return &APIServer{
		app:            app,
		cache:          cache.New(5*time.Minute, 10*time.Minute),
		wsHub:          ws.NewHub(logger),
		sseBroadcaster: sse.NewBroadcaster(logger),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(_ *http.Request) bool {
				return true // Allow all origins for WebSocket
			},
		},
	}, nil
}

// Start starts background services (WebSocket hub, SSE broadcaster).
func (s *APIServer) Start() {
	go s.wsHub.Run()
	go s.sseBroadcaster.Run()
}

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
func (s *APIServer) HandleListModels(w http.ResponseWriter, r *http.Request) {
	// Check cache
	cacheKey := "models:" + r.URL.RawQuery
	if cached, found := s.cache.Get(cacheKey); found {
		response.OK(w, cached)
		return
	}

	// Get catalog
	cat, err := s.app.Catalog()
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
	s.cache.Set(cacheKey, result)

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
func (s *APIServer) HandleGetModel(w http.ResponseWriter, _ *http.Request, modelID string) {
	// Check cache
	cacheKey := "model:" + modelID
	if cached, found := s.cache.Get(cacheKey); found {
		response.OK(w, cached)
		return
	}

	// Get catalog
	cat, err := s.app.Catalog()
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
	s.cache.Set(cacheKey, model)

	response.OK(w, model)
}

// SearchRequest represents the POST /api/v1/models/search request body.
type SearchRequest struct {
	IDs            []string          `json:"ids,omitempty"`
	NameContains   string            `json:"name_contains,omitempty"`
	Provider       string            `json:"provider,omitempty"`
	Modalities     *SearchModalities `json:"modalities,omitempty"`
	Features       map[string]bool   `json:"features,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	OpenWeights    *bool             `json:"open_weights,omitempty"`
	ContextWindow  *IntRange         `json:"context_window,omitempty"`
	OutputTokens   *IntRange         `json:"output_tokens,omitempty"`
	ReleaseDate    *DateRange        `json:"release_date,omitempty"`
	Sort           string            `json:"sort,omitempty"`
	Order          string            `json:"order,omitempty"`
	MaxResults     int               `json:"max_results,omitempty"`
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
func (s *APIServer) HandleSearchModels(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "Invalid JSON request body", err.Error())
		return
	}

	// Get catalog
	cat, err := s.app.Catalog()
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
func (s *APIServer) HandleListProviders(w http.ResponseWriter, _ *http.Request) {
	// Check cache
	if cached, found := s.cache.Get("providers"); found {
		response.OK(w, cached)
		return
	}

	// Get catalog
	cat, err := s.app.Catalog()
	if err != nil {
		response.InternalError(w, err)
		return
	}

	providers := cat.Providers().List()

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

		if prov.Catalog != nil && prov.Catalog.Docs != nil {
			providerInfo["docs_url"] = *prov.Catalog.Docs
		}

		providerList = append(providerList, providerInfo)
	}

	result := map[string]any{
		"providers": providerList,
		"count":     len(providerList),
	}

	// Cache result
	s.cache.Set("providers", result)

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
func (s *APIServer) HandleGetProvider(w http.ResponseWriter, _ *http.Request, providerID string) {
	// Check cache
	cacheKey := "provider:" + providerID
	if cached, found := s.cache.Get(cacheKey); found {
		response.OK(w, cached)
		return
	}

	// Get catalog
	cat, err := s.app.Catalog()
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

	// Cache result
	s.cache.Set(cacheKey, prov)

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
func (s *APIServer) HandleGetProviderModels(w http.ResponseWriter, _ *http.Request, providerID string) {
	// Get catalog
	cat, err := s.app.Catalog()
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

	// Convert map to slice
	models := make([]*catalogs.Model, 0, len(prov.Models))
	for _, model := range prov.Models {
		models = append(models, model)
	}

	result := map[string]any{
		"provider": map[string]any{
			"id":   prov.ID,
			"name": prov.Name,
		},
		"models": models,
		"count":  len(models),
	}

	response.OK(w, result)
}

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
func (s *APIServer) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	providerFilter := r.URL.Query().Get("provider")

	sm, err := s.app.Starmap()
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
	s.cache.Clear()

	// Broadcast update event
	s.broadcastEvent("sync.completed", map[string]any{
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

// HandleHealth handles GET /api/v1/health.
// @Summary Health check
// @Description Health check endpoint (liveness probe)
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} response.Response{data=object}
// @Router /api/v1/health [get].
func (s *APIServer) HandleHealth(w http.ResponseWriter, _ *http.Request) {
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
func (s *APIServer) HandleReady(w http.ResponseWriter, _ *http.Request) {
	// Check catalog availability
	_, err := s.app.Catalog()
	if err != nil {
		response.ServiceUnavailable(w, "Catalog not available")
		return
	}

	response.OK(w, map[string]any{
		"status": "ready",
		"cache": map[string]any{
			"items": s.cache.ItemCount(),
		},
		"websocket_clients": s.wsHub.ClientCount(),
		"sse_clients":       s.sseBroadcaster.ClientCount(),
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
func (s *APIServer) HandleStats(w http.ResponseWriter, _ *http.Request) {
	cat, err := s.app.Catalog()
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
		"cache": s.cache.GetStats(),
		"realtime": map[string]any{
			"websocket_clients": s.wsHub.ClientCount(),
			"sse_clients":       s.sseBroadcaster.ClientCount(),
		},
	})
}

// HandleWebSocket handles WebSocket connections at /api/v1/updates/ws.
// @Summary WebSocket updates
// @Description WebSocket connection for real-time catalog updates
// @Tags updates
// @Success 101 "Switching Protocols"
// @Router /api/v1/updates/ws [get].
func (s *APIServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.app.Logger().Error().Err(err).Msg("WebSocket upgrade failed")
		return
	}

	// Create client
	clientID := fmt.Sprintf("%s-%d", r.RemoteAddr, time.Now().Unix())
	client := ws.NewClient(clientID, s.wsHub, conn)

	// Register client
	s.wsHub.Broadcast(ws.Message{
		Type:      "client.connected",
		Timestamp: time.Now(),
		Data: map[string]any{
			"message": "Client connected to Starmap updates",
		},
	})

	// Start client pumps
	go client.WritePump()
	go client.ReadPump()
}

// HandleSSE handles Server-Sent Events at /api/v1/updates/stream.
// @Summary SSE updates stream
// @Description Server-Sent Events stream for catalog change notifications
// @Tags updates
// @Produce text/event-stream
// @Success 200 "Event stream"
// @Router /api/v1/updates/stream [get].
func (s *APIServer) HandleSSE(w http.ResponseWriter, r *http.Request) {
	s.sseBroadcaster.ServeHTTP(w, r)
}

// broadcastEvent sends an event to both WebSocket and SSE clients.
func (s *APIServer) broadcastEvent(eventType string, data any) {
	timestamp := time.Now()

	// WebSocket
	s.wsHub.Broadcast(ws.Message{
		Type:      eventType,
		Timestamp: timestamp,
		Data:      data,
	})

	// SSE
	s.sseBroadcaster.Broadcast(sse.Event{
		Event: eventType,
		ID:    fmt.Sprintf("%d", timestamp.Unix()),
		Data:  data,
	})
}

// Helper function to extract path parameter from URL.
func extractPathParam(path, prefix string) string {
	trimmed := strings.TrimPrefix(path, prefix)
	parts := strings.Split(trimmed, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// HandleOpenAPIJSON serves the embedded OpenAPI 3.0 specification in JSON format.
// @Summary Get OpenAPI specification (JSON)
// @Description Returns the OpenAPI 3.0 specification for this API in JSON format
// @Tags meta
// @Produce json
// @Success 200 {object} object "OpenAPI 3.0 specification"
// @Router /api/v1/openapi.json [get].
func (s *APIServer) HandleOpenAPIJSON(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	_, _ = w.Write(openapi.SpecJSON)
}

// HandleOpenAPIYAML serves the embedded OpenAPI 3.0 specification in YAML format.
// @Summary Get OpenAPI specification (YAML)
// @Description Returns the OpenAPI 3.0 specification for this API in YAML format
// @Tags meta
// @Produce application/x-yaml
// @Success 200 {string} string "OpenAPI 3.0 specification"
// @Router /api/v1/openapi.yaml [get].
func (s *APIServer) HandleOpenAPIYAML(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	_, _ = w.Write(openapi.SpecYAML)
}
