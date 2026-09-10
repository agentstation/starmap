package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/agentstation/starmap/internal/server/response"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/remote"
)

type permissionReader interface {
	ReadPermission(context.Context) (catalogs.CatalogPermissionEnvelope, error)
}

// HandleCatalogPermission serves a finite receipt independently of catalog availability.
// The reader owns issuance or relay validation. This handler never creates or renews a receipt.
// @Summary Current catalog permission receipt
// @Description Returns the confirmed upstream receipt with its original expiry. Catalog activation is not required. Unavailable permission returns 503.
// @Tags catalog
// @Produce application/vnd.agentstation.starmap.catalog-permission+json
// @Success 200 {object} catalogs.CatalogPermissionEnvelope
// @Failure 401 {object} response.Response
// @Failure 503 {object} response.Response
// @Router /api/v1/catalog/permission [get].
func (h *Handlers) HandleCatalogPermission(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		response.MethodNotAllowed(writer, request.Method)
		return
	}
	reader, ok := h.app.(permissionReader)
	if !ok {
		permissionUnavailable(writer)
		return
	}
	receipt, err := reader.ReadPermission(request.Context())
	if err != nil || receipt.Validate() != nil {
		permissionUnavailable(writer)
		return
	}
	data, err := json.Marshal(receipt)
	if err != nil || len(data) > catalogs.MaxCatalogPermissionEnvelopeBytes {
		permissionUnavailable(writer)
		return
	}
	writer.Header().Set("Content-Type", remote.PermissionEnvelopeMediaType)
	_, _ = writer.Write(data)
}

func permissionUnavailable(writer http.ResponseWriter) {
	response.JSON(writer, http.StatusServiceUnavailable, response.Fail(
		"catalog_permission_unavailable", "A current permission receipt is unavailable", "",
	))
}
