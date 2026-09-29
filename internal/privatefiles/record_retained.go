package privatefiles

import (
	"encoding/hex"
	"strings"
	"unicode"
	"unicode/utf8"
)

// RetainedPublication describes staging evidence in a captured publication journal.
// Native identities remain historical. This result cannot authorize native cleanup.
type RetainedPublication struct {
	Destination string
	Prefix      string
	Stage       string
	HasRecord   bool
	Size        int64
	SHA256      string
}

// RetainedPublicationMaxBytes bounds one captured publication journal.
const RetainedPublicationMaxBytes = publicationJournalMaxBytes

// InspectRetainedPublication validates a captured journal without accessing files.
// The caller must verify the backup, destination ownership, and any retained staging bytes.
// It must preserve the evidence and never promote staging bytes from this result.
func InspectRetainedPublication(name string, raw []byte) (RetainedPublication, error) {
	var result RetainedPublication
	nonce, ok := strings.CutSuffix(name, publicationJournalSuffix)
	if childName(name) != nil || !ok || len(nonce) != 26 || strings.Trim(nonce, "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567") != "" {
		return result, changed(name)
	}
	journal, err := decodePublicationJournal(name, raw)
	if err != nil {
		return result, err
	}
	h := journal.header
	if h.Version != publicationJournalVersion || childName(h.Destination) != nil || childName(h.Prefix) != nil || childName(h.Stage) != nil || h.Stage != h.Prefix+nonce || h.Stage == h.Destination || h.Destination == publicationDirectory {
		return result, changed(name)
	}
	for _, entry := range []publicationEntry{h.Parent, h.Metadata, h.Owner.Entry, h.Journal} {
		if !validRetainedEntry(entry) {
			return result, changed(name)
		}
	}
	if !validPublicationRecord(h.Owner) || h.Owner.Size != 0 || h.Owner.Digest != publicationDigest(nil) {
		return result, changed(name)
	}
	result = RetainedPublication{Destination: h.Destination, Prefix: h.Prefix, Stage: h.Stage}
	if journal.state != nil {
		if !validRetainedEntry(journal.state.Entry) || !validRetainedDigest(journal.state.Digest) {
			return RetainedPublication{}, changed(name)
		}
		result.HasRecord, result.Size, result.SHA256 = true, journal.state.Size, journal.state.Digest
	}
	return result, nil
}

func validRetainedEntry(entry publicationEntry) bool {
	return entry.Identity != "" && len(entry.Identity) <= 4096 && utf8.ValidString(entry.Identity) && !strings.ContainsFunc(entry.Identity, unicode.IsControl) && validRetainedDigest(entry.Access)
}

func validRetainedDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == value
}
