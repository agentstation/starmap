package main

import (
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/agentstation/starmap/pkg/catalogs/artifact"
	pkgerrors "github.com/agentstation/starmap/pkg/errors"
)

// maxRollbackListBytes bounds the release listing that the publisher supplies.
const maxRollbackListBytes = 4 << 20

// releaseListing is one entry of the GitHub release listing.
type releaseListing struct {
	TagName      string `json:"tagName"`
	IsPrerelease bool   `json:"isPrerelease"`
	IsDraft      bool   `json:"isDraft"`
	PublishedAt  string `json:"publishedAt"`
}

// rollbackReport lists the rollback candidates in newest-first order.
type rollbackReport struct {
	Candidates []rollbackCandidate `json:"candidates"`
}

// rollbackCandidate names one readable immutable release and its namespace.
type rollbackCandidate struct {
	Tag         string                `json:"tag"`
	Namespace   artifact.TagNamespace `json:"namespace"`
	PublishedAt string                `json:"published_at"`
}

// selectRollbackCandidates reads a GitHub release listing and returns every
// readable immutable catalog release, newest first. It keeps the canonical
// namespace and both retired namespaces, so every historical release stays a
// rollback target. It never returns the mutable channel or an unrelated tag.
func selectRollbackCandidates(listingPath, excludeTag string) (rollbackReport, error) {
	path := strings.TrimSpace(listingPath)
	if path == "" {
		return rollbackReport{}, rollbackError("rollback_candidates", "is required")
	}
	data, err := os.ReadFile(path) //nolint:gosec // the publisher selects the release listing path.
	if err != nil {
		return rollbackReport{}, pkgerrors.WrapIO("read", path, err)
	}
	if len(data) > maxRollbackListBytes {
		return rollbackReport{}, rollbackError("rollback_candidates", "exceeds the release listing size limit")
	}
	var listings []releaseListing
	if err := json.Unmarshal(data, &listings); err != nil {
		return rollbackReport{}, &pkgerrors.ParseError{
			File:    path,
			Message: "release listing is not a JSON array",
			Err:     err,
		}
	}

	excluded := strings.TrimSpace(excludeTag)
	candidates := make([]rollbackCandidate, 0, len(listings))
	for _, listing := range listings {
		namespace := artifact.ReleaseTagNamespace(listing.TagName)
		if namespace == artifact.NamespaceUnknown || listing.IsDraft ||
			!listing.IsPrerelease || listing.TagName == excluded {
			continue
		}
		candidates = append(candidates, rollbackCandidate{
			Tag:         listing.TagName,
			Namespace:   namespace,
			PublishedAt: listing.PublishedAt,
		})
	}
	sort.SliceStable(candidates, func(first, second int) bool {
		if candidates[first].PublishedAt == candidates[second].PublishedAt {
			return candidates[first].Tag > candidates[second].Tag
		}
		return candidates[first].PublishedAt > candidates[second].PublishedAt
	})
	return rollbackReport{Candidates: candidates}, nil
}

func rollbackError(field, message string) error {
	return &pkgerrors.ValidationError{Field: "catalog_release." + field, Message: message}
}
