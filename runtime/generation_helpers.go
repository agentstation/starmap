package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"

	"github.com/agentstation/starmap"
	"github.com/agentstation/starmap/internal/catalog/reconciler"
	"github.com/agentstation/starmap/pkg/catalogs"
	"github.com/agentstation/starmap/pkg/catalogs/evidence"
	"github.com/agentstation/starmap/pkg/errors"
)

// retainEmbeddedStartup stores an unchanged compiled baseline during ordinary startup.
// Root client construction remains read-only.
// Fleet, authority, origin, and pin publication retain separate authorization contracts.
func (r *Runtime) retainEmbeddedStartup(ctx context.Context, state starmap.CatalogState) error {
	if r.config.fleetStore != nil || r.lease.status() == leaseLost || !r.client.PublishesDurably() || !r.client.Readiness().Embedded.Active {
		return nil
	}
	baseline := r.client.EmbeddedCatalogState()
	if state.GenerationID != baseline.GenerationID || state.PayloadChecksum != baseline.PayloadChecksum ||
		!state.GeneratedAt.Equal(baseline.GeneratedAt) || state.AuthorityHead != baseline.AuthorityHead {
		return nil
	}
	generation, err := r.client.CurrentGeneration(ctx)
	if err != nil {
		return err
	}
	if err := r.lease.fence(r.lease.epoch()); err != nil {
		return err
	}
	ctx, err = r.prepareLocalRecovery(ctx, r.layers, nil)
	if err != nil {
		return err
	}
	// Activate preserves the immutable identity and bytes.
	// The store's CAS is idempotent on restart and refuses a conflicting retained head.
	if _, err := r.client.Activate(ctx, generation); err != nil {
		return err
	}
	r.mu.Lock()
	r.effective = r.client.CurrentCatalogState()
	r.mu.Unlock()
	return nil
}

// effectiveEvidenceChecksum binds catalog bytes and publication evidence to one identity.
// A receipt can change without supplying a selected catalog field.
func effectiveEvidenceChecksum(payloadChecksum string, candidate starmap.CandidateEvidence) (string, error) {
	if len(candidate.SourceObservations) == 0 && len(candidate.ReviewCandidates) == 0 {
		return payloadChecksum, nil
	}
	observations := append([]catalogs.SourceObservationLink{}, candidate.SourceObservations...)
	slices.SortFunc(observations, func(left, right catalogs.SourceObservationLink) int {
		if order := strings.Compare(left.Source.String(), right.Source.String()); order != 0 {
			return order
		}
		return strings.Compare(left.ObservationID, right.ObservationID)
	})
	reviews := append([]evidence.ReviewCandidate{}, candidate.ReviewCandidates...)
	slices.SortFunc(reviews, evidence.CompareReviewCandidates)
	raw, err := json.Marshal(struct {
		Domain          string
		PayloadChecksum string
		Observations    []catalogs.SourceObservationLink
		Reviews         []evidence.ReviewCandidate
	}{"starmap-effective-evidence:v1", payloadChecksum, observations, reviews})
	if err != nil {
		return "", errors.WrapResource("encode", "effective generation evidence", "", err)
	}
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func (l *layerSet) attachMembershipScopes(builder *catalogs.Builder, membership *reconciler.MembershipState) error {
	records, err := membership.ExportScopes(l.publisherID)
	if err != nil {
		return err
	}
	selected := make(map[string]bool, len(records))
	for _, record := range records {
		selected[record.BindingID] = true
	}
	var scopes []catalogs.ProviderMembershipScope
	for _, scope := range builder.MembershipScopes() {
		if scope.PublisherID != l.publisherID || !selected[scope.BindingID] {
			scopes = append(scopes, scope)
		}
	}
	return builder.SetMembershipScopes(append(scopes, records...))
}
