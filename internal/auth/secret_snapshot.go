package auth

import "maps"

type secretField struct {
	value string
	kind  SourceErrorKind
}

// secretSnapshot retains one immutable backend response during profile resolution.
type secretSnapshot struct {
	payload string
	fields  map[string]secretField
}

func (s *secretSnapshot) copy() *secretSnapshot {
	if s == nil {
		return nil
	}
	return &secretSnapshot{payload: s.payload, fields: maps.Clone(s.fields)}
}

func (s *secretSnapshot) selectValue(reference Reference) (string, error) {
	if s.fields == nil {
		if reference.field == "" {
			return s.payload, nil
		}
		value, found, err := selectJSONStringField([]byte(s.payload), reference.field)
		if err != nil {
			return "", newSourceError(SourceErrorInvalid, reference.backend)
		}
		if !found || value == "" {
			return "", newSourceError(SourceErrorNotConfigured, reference.backend)
		}
		return value, nil
	}
	if reference.field == "" {
		if len(s.fields) != 1 {
			return "", newSourceError(SourceErrorInvalid, reference.backend)
		}
		for _, field := range s.fields {
			return field.resolve(reference.backend)
		}
	}
	field, found := s.fields[reference.field]
	if !found {
		return "", newSourceError(SourceErrorNotConfigured, reference.backend)
	}
	return field.resolve(reference.backend)
}

func (f secretField) resolve(backend ReferenceBackend) (string, error) {
	if f.kind != "" {
		return "", newSourceError(f.kind, backend)
	}
	return f.value, nil
}

func newKVSecretSnapshot(data map[string]any) *secretSnapshot {
	fields := make(map[string]secretField, len(data))
	for name, value := range data {
		text, valid := value.(string)
		field := secretField{value: text}
		switch {
		case value == nil:
			field.kind = SourceErrorNotConfigured
		case !valid:
			field.kind = SourceErrorInvalid
		case text == "":
			field.kind = SourceErrorNotConfigured
		}
		fields[name] = field
	}
	return &secretSnapshot{fields: fields}
}
