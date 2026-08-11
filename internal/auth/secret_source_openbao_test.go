package auth

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	openbao "github.com/openbao/openbao/api/v2"
)

func TestOpenBaoSourceResolvesVersionedFieldAndCloses(t *testing.T) {
	var closed atomic.Int32
	source := &openBaoSource{open: func() (openBaoSecretRead, func() error, error) {
		read := func(_ context.Context, mount, path string, version int) (*openbao.KVSecret, error) {
			if mount != "secret" || path != "apps/catalog" || version != 7 {
				t.Fatalf("read = %q, %q, %d", mount, path, version)
			}
			return &openbao.KVSecret{
				Data:            map[string]any{"api-key": "valid-openbao", "other": "preserved"},
				VersionMetadata: &openbao.KVVersionMetadata{Version: 7},
			}, nil
		}
		return read, func() error { closed.Add(1); return nil }, nil
	}}

	material, err := source.Resolve(
		context.Background(), mustReference(t, "openbao:secret/apps/catalog?version=7#api-key"),
	)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if material.values["api-key"] != "valid-openbao" || material.version != "7" {
		t.Fatalf("material = %#v", material)
	}
	if closed.Load() != 1 {
		t.Fatalf("close calls = %d, want 1", closed.Load())
	}
}

func TestOpenBaoSourceValidatesReferenceDataAndErrors(t *testing.T) {
	for _, value := range []string{
		"openbao:secret", "openbao:secret/../key", "openbao:secret/key?version=0",
	} {
		source := &openBaoSource{open: func() (openBaoSecretRead, func() error, error) {
			t.Fatal("invalid reference opened a client")
			return nil, nil, nil
		}}
		_, err := source.Resolve(context.Background(), mustReference(t, value))
		if !isSourceError(err, SourceErrorInvalid) {
			t.Fatalf("Resolve(%q) error = %v", value, err)
		}
	}

	for _, test := range []struct {
		status int
		kind   SourceErrorKind
	}{
		{status: http.StatusNotFound, kind: SourceErrorNotConfigured},
		{status: http.StatusForbidden, kind: SourceErrorDenied},
		{status: http.StatusBadRequest, kind: SourceErrorInvalid},
		{status: http.StatusServiceUnavailable, kind: SourceErrorUnavailable},
	} {
		err := &openbao.ResponseError{StatusCode: test.status}
		if got := classifyOpenBaoSecretError(err); got != test.kind {
			t.Fatalf("status %d = %s, want %s", test.status, got, test.kind)
		}
	}
}
