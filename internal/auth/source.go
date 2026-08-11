package auth

import (
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/agentstation/starmap/pkg/sources"
)

const maxCredentialFileBytes = 1 << 20

type sourceMaterial struct {
	values    map[string]string
	version   string
	expiresAt time.Time
	lease     *sources.ProviderCredentialLease
}

func (m sourceMaterial) copy() sourceMaterial {
	copied := m
	copied.values = make(map[string]string, len(m.values))
	for key, value := range m.values {
		copied.values[key] = value
	}
	if m.lease != nil {
		lease := *m.lease
		copied.lease = &lease
	}
	return copied
}

func (m sourceMaterial) fresh(now time.Time) bool {
	if m.lease == nil || m.lease.RefreshAfter.IsZero() || !now.Before(m.lease.RefreshAfter) {
		return false
	}
	return m.expiresAt.IsZero() || now.Before(m.expiresAt)
}

type credentialSource interface {
	Backend() ReferenceBackend
	Resolve(context.Context, Reference) (sourceMaterial, error)
}

type environmentSource struct {
	lookup environmentLookup
}

func (environmentSource) Backend() ReferenceBackend { return referenceBackendEnvironment }

func (s environmentSource) Resolve(
	ctx context.Context,
	reference Reference,
) (sourceMaterial, error) {
	if err := ctx.Err(); err != nil {
		return sourceMaterial{}, err
	}
	if reference.field != "" || reference.version != "" ||
		!credentialEnvironmentPattern.MatchString(reference.resource) {
		return sourceMaterial{}, newSourceError(SourceErrorInvalid, s.Backend())
	}
	value, found := s.lookup(reference.resource)
	if !found || value == "" {
		return sourceMaterial{}, newSourceError(SourceErrorNotConfigured, s.Backend())
	}
	return sourceMaterial{
		values:  map[string]string{"value": value},
		version: reference.resource + "\x00" + value,
	}, nil
}

type fileSource struct{}

func (fileSource) Backend() ReferenceBackend { return referenceBackendFile }

func (s fileSource) Resolve(
	ctx context.Context,
	reference Reference,
) (sourceMaterial, error) {
	if err := ctx.Err(); err != nil {
		return sourceMaterial{}, err
	}
	if reference.field != "" || reference.version != "" ||
		!filepath.IsAbs(reference.resource) {
		return sourceMaterial{}, newSourceError(SourceErrorInvalid, s.Backend())
	}
	file, err := os.Open(reference.resource) // #nosec G304 -- the operator explicitly selects the credential file.
	if err != nil {
		return sourceMaterial{}, classifySourceIOError(s.Backend(), err)
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return sourceMaterial{}, classifySourceIOError(s.Backend(), err)
	}
	if !info.Mode().IsRegular() {
		return sourceMaterial{}, newSourceError(SourceErrorInvalid, s.Backend())
	}
	data, err := io.ReadAll(io.LimitReader(file, maxCredentialFileBytes+1))
	if err != nil {
		return sourceMaterial{}, classifySourceIOError(s.Backend(), err)
	}
	if err := ctx.Err(); err != nil {
		return sourceMaterial{}, err
	}
	if len(data) > maxCredentialFileBytes {
		return sourceMaterial{}, newSourceError(SourceErrorInvalid, s.Backend())
	}
	if len(data) == 0 {
		return sourceMaterial{}, newSourceError(SourceErrorNotConfigured, s.Backend())
	}
	return sourceMaterial{
		values:  map[string]string{"value": string(data)},
		version: string(data),
	}, nil
}

// SourceErrorKind classifies a secret-free credential source failure.
type SourceErrorKind string

const (
	// SourceErrorNotConfigured means that the selected source has no material.
	SourceErrorNotConfigured SourceErrorKind = "not_configured"
	// SourceErrorDenied means that source access or authentication was denied.
	SourceErrorDenied SourceErrorKind = "denied"
	// SourceErrorInvalid means that the source reference or material is invalid.
	SourceErrorInvalid SourceErrorKind = "invalid"
	// SourceErrorUnavailable means that a configured source could not complete.
	SourceErrorUnavailable SourceErrorKind = "unavailable"
)

type sourceError struct {
	kind    SourceErrorKind
	backend ReferenceBackend
}

func newSourceError(kind SourceErrorKind, backend ReferenceBackend) error {
	return &sourceError{kind: kind, backend: backend}
}

func (e *sourceError) Error() string {
	return fmt.Sprintf("credential source %s is %s", e.backend, e.kind)
}

func isSourceError(err error, kind SourceErrorKind) bool {
	var sourceErr *sourceError
	return stderrors.As(err, &sourceErr) && sourceErr.kind == kind
}

func classifySourceIOError(backend ReferenceBackend, err error) error {
	switch {
	case stderrors.Is(err, os.ErrNotExist):
		return newSourceError(SourceErrorNotConfigured, backend)
	case stderrors.Is(err, os.ErrPermission):
		return newSourceError(SourceErrorDenied, backend)
	default:
		return newSourceError(SourceErrorUnavailable, backend)
	}
}
