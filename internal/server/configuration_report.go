package server

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/agentstation/starmap/server/administration"
)

// AdministrativeStatus is the wire report for standalone administrative readiness.
type AdministrativeStatus = administration.Status

func (s *Server) registerAdministrationReports(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/status", s.administrationStatus)
	mux.HandleFunc("GET /admin/config/schema", s.configurationSchema)
	mux.HandleFunc("GET /admin/config/effective", s.effectiveConfiguration)
}

// administrationStatus reports audit readiness without reading catalog state.
// @Summary Read administration readiness
// @Description Requires administrator permission. Remains available during an audit-write failure.
// @Tags administration
// @Produce json
// @Success 200 {object} AdministrativeStatus
// @Security ApiKeyAuth
// @Router /admin/status [get].
func (s *Server) administrationStatus(w http.ResponseWriter, _ *http.Request) {
	if s.administration == nil {
		http.Error(w, "Administration unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	_ = json.NewEncoder(w).Encode(s.administration.Status())
}

// configurationSchema reports catalog setting descriptors without starting the runtime.
// @Summary Read the configuration schema
// @Description Requires administrator permission. Uses the same versioned descriptors as starmap config schema.
// @Tags administration
// @Produce json
// @Success 200 {object} object
// @Security ApiKeyAuth
// @Router /admin/config/schema [get].
func (s *Server) configurationSchema(w http.ResponseWriter, r *http.Request) {
	s.writeConfigurationReport(w, r, false)
}

// effectiveConfiguration reports redacted values, origins, presence, and managed paths.
// @Summary Read effective configuration
// @Description Requires administrator permission. Works before internal catalog readiness and matches the CLI report.
// @Tags administration
// @Produce json
// @Success 200 {object} object
// @Security ApiKeyAuth
// @Router /admin/config/effective [get].
func (s *Server) effectiveConfiguration(w http.ResponseWriter, r *http.Request) {
	s.writeConfigurationReport(w, r, true)
}

func (s *Server) writeConfigurationReport(w http.ResponseWriter, r *http.Request, effective bool) {
	if s.reports == nil {
		http.Error(w, "Configuration report unavailable", http.StatusServiceUnavailable)
		return
	}
	data := s.reports.Schema()
	if effective {
		data = s.reports.Effective()
	}
	digest := sha256.Sum256(data)
	etag := fmt.Sprintf(`"%x"`, digest)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	_, _ = w.Write(append(data, '\n'))
}
