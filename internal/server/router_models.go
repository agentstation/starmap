package server

import (
	"net/http"

	"github.com/agentstation/starmap/internal/server/handlers"
	"github.com/agentstation/starmap/internal/server/openrouter"
)

func (s *Server) registerModelRoutes(
	mux *http.ServeMux,
	h *handlers.Handlers,
) {
	prefix := s.config.PathPrefix
	mux.HandleFunc(prefix+"/model/", func(w http.ResponseWriter, r *http.Request) {
		parts, err := extractExactPathSegments(r, prefix+"/model/", 2)
		if err != nil || len(parts) != 2 {
			openrouter.WriteError(w, http.StatusNotFound, "Resource not found")
			return
		}
		if r.Method != http.MethodGet {
			openrouter.WriteError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
			return
		}
		h.HandleOpenRouterModel(w, r, parts[0], parts[1], prefix)
	})

	mux.HandleFunc(prefix+"/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.HandleListModels(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	mux.HandleFunc(prefix+"/models/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == prefix+"/models/search" {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			h.HandleSearchModels(w, r)
			return
		}

		parts, pathErr := extractExactPathSegments(r, prefix+"/models/", 3)
		if pathErr != nil &&
			openrouter.IsCompatibilityPath(r.URL.EscapedPath(), prefix) {
			openrouter.WriteError(w, http.StatusNotFound, "Resource not found")
			return
		}
		if pathErr == nil && len(parts) == 3 && parts[2] == "endpoints" {
			if r.Method != http.MethodGet {
				openrouter.WriteError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
				return
			}
			h.HandleOpenRouterEndpoints(w, r, parts[0], parts[1])
			return
		}

		modelID, err := extractPathParam(r, prefix+"/models/")
		if err != nil {
			http.Error(w, "Invalid model ID", http.StatusBadRequest)
			return
		}
		if modelID != "" && r.Method == http.MethodGet {
			h.HandleGetModel(w, r, modelID)
			return
		}
		http.Error(w, "Not found", http.StatusNotFound)
	})
}
