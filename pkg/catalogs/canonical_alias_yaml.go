package catalogs

import (
	"github.com/goccy/go-yaml"

	"github.com/agentstation/starmap/pkg/errors"
)

const canonicalAliasFileName = "canonical-aliases.yaml"

func (b *Builder) loadCanonicalAliasesYAML() error {
	records, err := readWorkspaceRecords[CanonicalAlias](b.config.readFilesystem(), canonicalAliasFileName, "canonical_alias.workspace")
	if err != nil {
		return err
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
