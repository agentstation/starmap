package handlers

import (
	"net/http"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/server/response"
	protocol "github.com/agentstation/starmap/pkg/catalogs/remote"
	"github.com/agentstation/starmap/pkg/logging"
)

// HandleCatalogSourceChain handles GET /api/v1/catalog/source-chain.
// @Summary Source chain manifest
// @Description Safe identities and bounded detail for each cascade hop
// @Tags catalog
// @Produce json
// @Success 200 {object} object
// @Router /api/v1/catalog/source-chain [get].
func (h *Handlers) HandleCatalogSourceChain(w http.ResponseWriter, _ *http.Request) {
	chain := SourceChainOf(h.app.RuntimeStatus())
	data, err := protocol.MarshalSourceChain(chain)
	if err != nil {
		response.ServiceUnavailable(w, "Source chain not available")
		return
	}
	w.Header().Set("Content-Type", protocol.SourceChainMediaType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// SourceChainOf builds the served source-chain manifest from runtime status.
// The document carries safe identities and bounded detail only, so it names no
// URL, no credential, and no operator message. A downstream reads it to detect
// a cycle and to grade the propagated origin freshness.
func SourceChainOf(status starmap.RuntimeStatus) protocol.SourceChain {
	chain := protocol.SourceChain{
		SchemaVersion:    protocol.SourceChainSchemaVersion,
		Identity:         status.InstanceIdentity,
		Health:           sourceChainHealth(status.SourceHealth),
		UpstreamHealth:   sourceChainHealth(status.UpstreamHealth),
		SourceIdentity:   status.SourceIdentity,
		GenerationID:     status.GenerationID,
		ChannelUpdatedAt: status.ChannelUpdatedAt,
		ObservedAt:       status.ObservedAt,
	}
	// The served chain keeps the hop budget, so one node cannot grow a
	// document without bound by forwarding every hop it received.
	hops := status.Chain
	if len(hops) > protocol.MaxSourceChainHops-1 {
		// A downstream detects a cycle from the disclosed hops, so an operator
		// needs to know that this node served fewer hops than it observed.
		logging.Warn().
			Int("observed_hops", len(hops)).
			Int("served_hops", protocol.MaxSourceChainHops-1).
			Msg("Source chain served fewer hops than the runtime observed")
		hops = hops[:protocol.MaxSourceChainHops-1]
	}
	for _, hop := range hops {
		chain.Hops = append(chain.Hops, protocol.SourceChainHop{
			Identity:    hop.Identity,
			Health:      sourceChainHealth(hop.Health),
			PublishedAt: hop.PublishedAt,
			ObservedAt:  hop.ObservedAt,
		})
	}
	return chain
}

// sourceChainHealth converts one runtime health onto the closed chain code.
func sourceChainHealth(health starmap.Health) string {
	switch health {
	case starmap.HealthOK:
		return protocol.SourceChainHealthOK
	case starmap.HealthDegraded:
		return protocol.SourceChainHealthDegraded
	case starmap.HealthUnavailable:
		return protocol.SourceChainHealthUnavailable
	default:
		return protocol.SourceChainHealthUnknown
	}
}
