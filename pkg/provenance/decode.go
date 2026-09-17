package provenance

import (
	"bytes"

	"github.com/agentstation/starmap/pkg/errors"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/token"
)

const provenanceBatchEntries = 128

// DecodeYAML reads provenance without changing the YAML scalar rules.
// It parses independent entries in the generated block map in bounded batches.
// Other layouts and documents with anchors use the original whole-file parser.
func DecodeYAML(data []byte) (*File, error) {
	boundaries := provenanceBatchBoundaries(data)
	if len(boundaries) < 2 {
		return decodeWholeFile(data)
	}
	result := &File{Provenance: make(Map)}
	for index, start := range boundaries {
		end := len(data)
		if index+1 < len(boundaries) {
			end = boundaries[index+1]
		}
		batch := append([]byte("provenance:\n"), data[start:end]...)
		if hasDependentYAML(batch) {
			return decodeWholeFile(data)
		}
		part, err := decodeWholeFile(batch)
		if err != nil {
			// Preserve whole-file diagnostics and support dependent YAML forms.
			return decodeWholeFile(data)
		}
		for key, entries := range part.Provenance {
			if _, exists := result.Provenance[key]; exists {
				return decodeWholeFile(data)
			}
			result.Provenance[key] = entries
		}
	}
	return result, nil
}

func decodeWholeFile(data []byte) (*File, error) {
	var file File
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, errors.WrapParse("yaml", "", err)
	}
	return &file, nil
}

func provenanceBatchBoundaries(data []byte) []int {
	const header = "provenance:\n"
	if !bytes.HasPrefix(data, []byte(header)) {
		return nil
	}
	boundaries := []int{len(header)}
	entries := 0
	offset := len(header)
	for line := range bytes.SplitSeq(data[len(header):], []byte("\n")) {
		if len(line) > 0 && line[0] != ' ' && line[0] != '#' && line[0] != '\r' {
			return nil
		}
		if len(line) > 2 && bytes.HasPrefix(line, []byte("  ")) &&
			line[2] != ' ' && line[2] != '-' && line[2] != '#' &&
			bytes.HasSuffix(bytes.TrimSpace(line), []byte(":")) {
			if entries > 0 && entries%provenanceBatchEntries == 0 {
				boundaries = append(boundaries, offset)
			}
			entries++
		}
		offset += len(line) + 1
	}
	return boundaries
}

func hasDependentYAML(data []byte) bool {
	for _, item := range lexer.Tokenize(string(data)) {
		switch item.Type {
		case token.InvalidType, token.AnchorType, token.AliasType, token.TagType,
			token.DirectiveType, token.DocumentHeaderType, token.DocumentEndType, token.MappingKeyType:
			return true
		}
	}
	return false
}
