package source

import "github.com/agentstation/starmap/pkg/catalogs/artifact"

// Publication binds a verified run receipt to its attested channel metadata.
// Source implementations verify publisher provenance before returning this record.
type Publication struct {
	Receipt      artifact.PublicationReceipt `json:"receipt"`
	Checksum     string                      `json:"checksum"`
	SourceCommit string                      `json:"source_commit"`
}

// Copy returns an independent publication record.
func (p Publication) Copy() Publication {
	p.Receipt = p.Receipt.Copy()
	return p
}
