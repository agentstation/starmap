package privatefiles

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

const (
	retainedPublicationMaxFiles     = 100000
	retainedPublicationMaxNameBytes = 4 << 20
)

// RetainedRecordReader reads bounded bytes from a verified backup.
// An absent record must return an error that matches os.ErrNotExist.
type RetainedRecordReader func(context.Context, string, int64) ([]byte, error)

// RetainedFile identifies bytes in a verified backup. Names are slash-separated relative paths.
type RetainedFile struct {
	Size   int64
	SHA256 string
}

// InspectRetainedPublications selects native journals and matching staging files to keep inactive.
// The caller supplies a complete verified inventory and a destination owner check.
// It must preserve every selected file in inactive recovery evidence.
// Destination records remain active and require separate owner validation. This check never promotes staging files.
// This result does not authorize native cleanup or admission.
func InspectRetainedPublications(ctx context.Context, files map[string]RetainedFile, read RetainedRecordReader, accepts func(parent, destination, prefix string) bool) ([]string, error) {
	if ctx == nil || read == nil || accepts == nil || len(files) > retainedPublicationMaxFiles {
		return nil, invalidRetainedPublication("retained publication requires bounded verified input")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	journals, err := retainedPublicationJournals(files)
	if err != nil {
		return nil, err
	}
	inactive := make(map[string]bool)
	for _, name := range journals {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		raw, err := read(ctx, name, RetainedPublicationMaxBytes)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(raw)
		if file := files[name]; file.Size != int64(len(raw)) || file.SHA256 != hex.EncodeToString(digest[:]) {
			return nil, invalidRetainedPublication("retained publication changed after backup verification")
		}
		journal, err := InspectRetainedPublication(path.Base(name), raw)
		if err != nil {
			return nil, err
		}
		parent := path.Dir(path.Dir(name))
		if !accepts(parent, journal.Destination, journal.Prefix) {
			return nil, invalidRetainedPublication("retained publication destination is not owned by the caller")
		}
		lock, exists := files[path.Join(path.Dir(name), publicationLock)]
		if !exists || lock.Size != 0 || lock.SHA256 != hex.EncodeToString(sha256.New().Sum(nil)) {
			return nil, invalidRetainedPublication("retained publication writer record is missing or invalid")
		}
		stage := path.Join(parent, journal.Stage)
		if file, exists := files[stage]; exists {
			if inactive[stage] || !journal.HasRecord || file.Size != journal.Size || file.SHA256 != journal.SHA256 {
				return nil, invalidRetainedPublication("retained staging bytes differ from the publication record")
			}
			inactive[stage] = true
		}
		inactive[name] = true
	}
	result := make([]string, 0, len(inactive))
	for name := range inactive {
		result = append(result, name)
	}
	slices.Sort(result)
	return result, ctx.Err()
}

func retainedPublicationJournals(files map[string]RetainedFile) ([]string, error) {
	var journals []string
	namesBytes := 0
	for name, file := range files {
		namesBytes += len(name)
		if !fs.ValidPath(name) || name == "." || strings.Contains(name, "\\") || namesBytes > retainedPublicationMaxNameBytes || file.Size < 0 || !validRetainedDigest(file.SHA256) {
			return nil, invalidRetainedPublication("retained publication inventory is invalid")
		}
		if path.Base(path.Dir(name)) == publicationDirectory && path.Base(name) != publicationLock {
			journals = append(journals, name)
		}
	}
	slices.Sort(journals)
	return journals, nil
}

func invalidRetainedPublication(message string) error {
	return &errors.ValidationError{Field: "private.retained_publication", Message: message}
}
