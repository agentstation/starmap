package runtime

import (
	"context"

	"github.com/agentstation/starmap/pkg/errors"
)

type authorityPublicationContextKey struct{}

// The nonzero capability has one identity per runtime. It never reaches source or provider callbacks.
type authorityPublicationCapability struct{ owned bool }

func (owner *authorityPublicationCapability) guard(ctx context.Context) error {
	capability, _ := ctx.Value(authorityPublicationContextKey{}).(*authorityPublicationCapability)
	if capability == nil || capability != owner || !capability.owned {
		return &errors.ConflictError{Resource: "catalog authority publication", Message: "the connected runtime owns publication into this client"}
	}
	return nil
}

func (r *Runtime) authorityPublicationContext(ctx context.Context) context.Context {
	if !r.requiresAuthority() && r.config.origin == nil {
		return ctx
	}
	return context.WithValue(ctx, authorityPublicationContextKey{}, r.config.publicationCapability)
}
