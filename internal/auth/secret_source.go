package auth

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"strings"
)

const maxCredentialPayloadBytes = 1 << 20

func defaultDirectSecretSources() []credentialSource {
	return []credentialSource{
		newGCPSecretManagerSource(),
		newAzureKeyVaultSource(),
		newAWSSecretsManagerSource(),
		newVaultSource(),
		newOpenBaoSource(),
	}
}

func scalarSecretMaterial(
	backend ReferenceBackend,
	payload []byte,
	field string,
	version string,
) (sourceMaterial, error) {
	if len(payload) == 0 {
		return sourceMaterial{}, newSourceError(SourceErrorNotConfigured, backend)
	}
	if len(payload) > maxCredentialPayloadBytes || version == "" {
		return sourceMaterial{}, newSourceError(SourceErrorInvalid, backend)
	}
	if field == "" {
		return sourceMaterial{
			values: map[string]string{"value": string(payload)}, version: version,
		}, nil
	}
	value, found, err := selectJSONStringField(payload, field)
	if err != nil {
		return sourceMaterial{}, newSourceError(SourceErrorInvalid, backend)
	}
	if !found || value == "" {
		return sourceMaterial{}, newSourceError(SourceErrorNotConfigured, backend)
	}
	return sourceMaterial{
		values: map[string]string{field: value}, version: version,
	}, nil
}

func selectJSONStringField(payload []byte, field string) (string, bool, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return "", false, errInvalidSecretObject
	}
	seen := make(map[string]struct{})
	selected := json.RawMessage(nil)
	for decoder.More() {
		keyToken, keyErr := decoder.Token()
		key, ok := keyToken.(string)
		if keyErr != nil || !ok {
			return "", false, errInvalidSecretObject
		}
		if _, exists := seen[key]; exists {
			return "", false, errInvalidSecretObject
		}
		seen[key] = struct{}{}
		var raw json.RawMessage
		if decodeErr := decoder.Decode(&raw); decodeErr != nil {
			return "", false, errInvalidSecretObject
		}
		if key == field {
			selected = append(selected[:0], raw...)
		}
	}
	if _, err = decoder.Token(); err != nil {
		return "", false, errInvalidSecretObject
	}
	if err = rejectTrailingJSON(decoder); err != nil {
		return "", false, err
	}
	if selected == nil {
		return "", false, nil
	}
	var value string
	if err = json.Unmarshal(selected, &value); err != nil {
		return "", false, errInvalidSecretObject
	}
	return value, true, nil
}

func rejectTrailingJSON(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !stderrors.Is(err, io.EOF) {
		return errInvalidSecretObject
	}
	return nil
}

var errInvalidSecretObject = stderrors.New("invalid secret object")

func sourceFailure(
	ctx context.Context,
	backend ReferenceBackend,
	err error,
	classify func(error) SourceErrorKind,
) error {
	if contextErr := ctx.Err(); contextErr != nil {
		return contextErr
	}
	if stderrors.Is(err, context.Canceled) || stderrors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return newSourceError(classify(err), backend)
}

func ownedHTTPClient() (*http.Client, func() error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{}, func() error { return nil }
	}
	ownedTransport := transport.Clone()
	client := &http.Client{
		Transport: ownedTransport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return client, func() error {
		ownedTransport.CloseIdleConnections()
		return nil
	}
}

func hasControlCharacter(value string) bool {
	return strings.IndexFunc(value, func(character rune) bool {
		return character < ' ' || character == 0x7f
	}) >= 0
}
