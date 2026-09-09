package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
)

type acquisitionSourcePolicy struct{ ids []sources.ID }

// WithAcquisitionSources selects permitted local acquisition and retained evidence.
// An explicit empty set disables every acquisition source. It preserves the selected baseline.
// Omission preserves the acquisition roles and retained inputs that the caller supplies.
func WithAcquisitionSources(ids ...sources.ID) Option {
	owned := slices.Clone(ids)
	return func(config *options) error {
		if err := sources.ValidateAcquisitionSelection(owned); err != nil {
			return err
		}
		selected := append([]sources.ID{}, owned...)
		slices.Sort(selected)
		config.acquisitionSources = &acquisitionSourcePolicy{ids: selected}
		return nil
	}
}

func (p *acquisitionSourcePolicy) permits(id sources.ID) bool {
	return p == nil || slices.Contains(p.ids, id)
}
func (p *acquisitionSourcePolicy) metadata() []sources.ID {
	if p == nil {
		return nil
	}
	return slices.DeleteFunc(append([]sources.ID{}, p.ids...), func(id sources.ID) bool { return id == sources.ProvidersID })
}
func (p *acquisitionSourcePolicy) permitsMetadata() bool {
	return p == nil || slices.ContainsFunc(p.ids, func(id sources.ID) bool { return id != sources.ProvidersID })
}
func (p *acquisitionSourcePolicy) validateManual(observations []manualObservation) error {
	for _, observation := range observations {
		if !p.permits(observation.Receipt.Link.Source) {
			return &errors.ConflictError{Resource: "acquisition source", Message: "observation comes from an excluded source"}
		}
	}
	return nil
}
func (p *acquisitionSourcePolicy) generationID(upstream, checksum string) (string, error) {
	raw, err := json.Marshal(struct {
		Domain, Upstream, Checksum string
		Sources                    []sources.ID
	}{"starmap-source-selection:v1", upstream, checksum, p.ids})
	if err != nil {
		return "", errors.WrapResource("encode", "acquisition source selection", "", err)
	}
	digest := sha256.Sum256(raw)
	return "sources-" + hex.EncodeToString(digest[:]), nil
}

// AcquisitionSources returns an owned explicit source set and its presence.
// Without a selection, the supplied acquisition roles keep their defaults.
func (r *Runtime) AcquisitionSources() ([]sources.ID, bool) {
	if r.config.acquisitionSources == nil {
		return nil, false
	}
	return append([]sources.ID{}, r.config.acquisitionSources.ids...), true
}
