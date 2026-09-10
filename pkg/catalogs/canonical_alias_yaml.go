package catalogs

import (
	stderrors "errors"
	"io"
	"io/fs"

	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/pkg/catalogs/internal/resourcepolicy"
	"github.com/agentstation/starmap/pkg/errors"
)

const canonicalAliasFileName = "canonical-aliases.yaml"

func (b *Builder) loadCanonicalAliasesYAML() error {
	file, err := b.config.readFilesystem().Open(canonicalAliasFileName)
	if stderrors.Is(err, fs.ErrNotExist) {
		return b.SetCanonicalAliasRecords(nil)
	}
	if err != nil {
		return errors.WrapIO("open", canonicalAliasFileName, err)
	}
	data, err := io.ReadAll(io.LimitReader(file, resourcepolicy.MaxPayloadBytes+1))
	if err := stderrors.Join(err, file.Close()); err != nil {
		return errors.WrapIO("read", canonicalAliasFileName, err)
	}
	if len(data) > resourcepolicy.MaxPayloadBytes {
		return invalidCanonicalAlias("workspace", "exceeds the catalog payload byte limit")
	}
	var records []CanonicalAlias
	if err := yaml.UnmarshalWithOptions(data, &records, yaml.Strict()); err != nil {
		return errors.WrapParse("yaml", canonicalAliasFileName, err)
	}
	if records == nil {
		return invalidCanonicalAlias("workspace", "requires an explicit record list")
	}
	return b.SetCanonicalAliasRecords(records)
}

func (b *Builder) saveCanonicalAliasesYAML(writeFile catalogFileWriter) error {
	records := b.CanonicalAliasRecords()
	if len(records) == 0 {
		return nil
	}
	data, err := yaml.Marshal(records)
	if err != nil {
		return errors.WrapResource("encode", "canonical aliases", "", err)
	}
	if err := writeFile(canonicalAliasFileName, data); err != nil {
		return errors.WrapIO("write", canonicalAliasFileName, err)
	}
	return nil
}
