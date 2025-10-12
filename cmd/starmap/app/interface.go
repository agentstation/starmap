package app

import (
	"github.com/agentstation/starmap/internal/appcontext"
)

// Interface is an alias to the shared appcontext.Interface.
// This maintains backward compatibility while using the centralized interface definition.
//
// Deprecated: New code should use internal/appcontext.Interface directly.
// This alias exists for backward compatibility during migration.
type Interface = appcontext.Interface

// Ensure App implements appcontext.Interface at compile time.
var _ appcontext.Interface = (*App)(nil)
