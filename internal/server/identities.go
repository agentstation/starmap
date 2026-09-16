package server

import (
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"time"

	"github.com/agentstation/starmap/internal/server/middleware"
	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/server/administration"
)

const maximumIdentityRequestBytes = 4096

// AdministrativeIdentityInput selects a new server identity and its role.
type AdministrativeIdentityInput struct {
	// ID names the identity within one catalog audience.
	ID string `json:"id"`
	// Role selects subscriber or administrator permission.
	Role administration.Role `json:"role"`
}

// AdministrativeRotationInput sets the bounded overlap for the previous credential.
type AdministrativeRotationInput struct {
	// Overlap is a duration between zero and 24 hours.
	Overlap string `json:"overlap"`
}

// AdministrativeCredential contains a new credential returned once to the administrator.
type AdministrativeCredential struct {
	// ID names the credential's identity.
	ID string `json:"id"`
	// Credential is secret and must not enter logs or shared caches.
	Credential string `json:"credential"`
}

func (s *Server) registerIdentityRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /admin/identities", s.createIdentity)
	mux.HandleFunc("POST /admin/identities/{id}/rotate", s.rotateIdentity)
	mux.HandleFunc("DELETE /admin/identities/{id}", s.revokeIdentity)
}

// createIdentity requires administrator permission and records a durable identity change.
// @Summary Create a server identity
// @Description Requires an administrator credential. Returns the new credential once after durable audit completion.
// @Tags administration
// @Accept json
// @Produce json
// @Param request body AdministrativeIdentityInput true "Identity and role"
// @Success 201 {object} AdministrativeCredential
// @Failure 400 {string} string "Invalid identity request"
// @Failure 403 {string} string "Administrator permission required"
// @Failure 409 {string} string "Identity exists"
// @Failure 503 {string} string "Administrative writes unavailable"
// @Security ApiKeyAuth
// @Router /admin/identities [post].
func (s *Server) createIdentity(w http.ResponseWriter, r *http.Request) {
	var input AdministrativeIdentityInput
	if !decodeIdentityRequest(w, r, &input) {
		return
	}
	actor, ok := middleware.AdministrativePrincipal(r.Context())
	if !ok || s.administration == nil {
		writeIdentityError(w, &errors.AuthenticationError{})
		return
	}
	token, err := s.administration.Create(r.Context(), actor, input.ID, input.Role)
	if err != nil {
		writeIdentityError(w, err)
		return
	}
	writeIdentityCredential(w, http.StatusCreated, input.ID, token)
}

// rotateIdentity replaces a server credential after recording its audit intent.
// @Summary Rotate a server credential
// @Description Requires administrator permission and an explicit overlap between zero and 24 hours.
// @Tags administration
// @Accept json
// @Produce json
// @Param id path string true "Identity name"
// @Param request body AdministrativeRotationInput true "Credential overlap"
// @Success 200 {object} AdministrativeCredential
// @Failure 403 {string} string "Administrator permission required"
// @Failure 409 {string} string "Prior overlap remains active"
// @Failure 503 {string} string "Administrative writes unavailable"
// @Security ApiKeyAuth
// @Router /admin/identities/{id}/rotate [post].
func (s *Server) rotateIdentity(w http.ResponseWriter, r *http.Request) {
	var input AdministrativeRotationInput
	if !decodeIdentityRequest(w, r, &input) {
		return
	}
	overlap, err := time.ParseDuration(input.Overlap)
	if err != nil {
		http.Error(w, "overlap requires a duration", http.StatusBadRequest)
		return
	}
	actor, ok := middleware.AdministrativePrincipal(r.Context())
	if !ok || s.administration == nil {
		writeIdentityError(w, &errors.AuthenticationError{})
		return
	}
	token, err := s.administration.Rotate(r.Context(), actor, r.PathValue("id"), overlap)
	if err != nil {
		writeIdentityError(w, err)
		return
	}
	writeIdentityCredential(w, http.StatusOK, r.PathValue("id"), token)
}

// revokeIdentity withdraws every credential for one identity.
// @Summary Revoke a server identity
// @Description Requires administrator permission. One administrator must remain active.
// @Tags administration
// @Param id path string true "Identity name"
// @Success 204 "Identity revoked"
// @Failure 403 {string} string "Administrator permission required"
// @Failure 409 {string} string "Last administrator cannot be revoked"
// @Failure 503 {string} string "Administrative writes unavailable"
// @Security ApiKeyAuth
// @Router /admin/identities/{id} [delete].
func (s *Server) revokeIdentity(w http.ResponseWriter, r *http.Request) {
	actor, ok := middleware.AdministrativePrincipal(r.Context())
	if !ok || s.administration == nil {
		writeIdentityError(w, &errors.AuthenticationError{})
		return
	}
	if err := s.administration.Revoke(r.Context(), actor, r.PathValue("id")); err != nil {
		writeIdentityError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusNoContent)
}

func decodeIdentityRequest(w http.ResponseWriter, r *http.Request, destination any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maximumIdentityRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		http.Error(w, "Invalid identity request", http.StatusBadRequest)
		return false
	}
	if _, err := decoder.Token(); err != io.EOF {
		http.Error(w, "Identity request requires one JSON object", http.StatusBadRequest)
		return false
	}
	return true
}

func writeIdentityCredential(w http.ResponseWriter, status int, id, token string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(AdministrativeCredential{ID: id, Credential: token})
}

func writeIdentityError(w http.ResponseWriter, err error) {
	var authentication *errors.AuthenticationError
	var validation *errors.ValidationError
	var missing *errors.NotFoundError
	var conflict *errors.ConflictError
	switch {
	case stderrors.As(err, &authentication):
		http.Error(w, "Administrator permission required", http.StatusForbidden)
	case stderrors.As(err, &validation):
		http.Error(w, "Invalid identity or rotation value", http.StatusBadRequest)
	case stderrors.As(err, &missing):
		http.Error(w, "Identity not found", http.StatusNotFound)
	case stderrors.As(err, &conflict):
		http.Error(w, "Identity change conflicts with current state", http.StatusConflict)
	default:
		http.Error(w, "Administrative change unavailable; inspect /admin/status", http.StatusServiceUnavailable)
	}
}
