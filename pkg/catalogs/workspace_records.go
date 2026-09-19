package catalogs

import (
	stderrors "errors"
	"io"
	"io/fs"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/pkg/catalogs/internal/resourcepolicy"
	"github.com/agentstation/starmap/pkg/errors"
)

// readWorkspaceRecords accepts an optional, bounded YAML list with strict fields.
func readWorkspaceRecords[T any](filesystem fs.FS, filename, field string) ([]T, error) {
	file, err := filesystem.Open(filename)
	if stderrors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.WrapIO("open", filename, err)
	}
	data, err := io.ReadAll(io.LimitReader(file, resourcepolicy.MaxPayloadBytes+1))
	if err := stderrors.Join(err, file.Close()); err != nil {
		return nil, errors.WrapIO("read", filename, err)
	}
	if len(data) > resourcepolicy.MaxPayloadBytes {
		return nil, &errors.ValidationError{Field: field, Message: "exceeds the catalog payload byte limit"}
	}
	var records []T
	if err := yaml.UnmarshalWithOptions(data, &records, yaml.Strict()); err != nil {
		return nil, errors.WrapParse("yaml", filename, err)
	}
	if records == nil {
		return nil, &errors.ValidationError{Field: field, Message: "requires an explicit record list"}
	}
	return records, nil
}
