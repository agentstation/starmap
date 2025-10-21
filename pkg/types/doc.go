// Package types provides shared type definitions used across the starmap packages.
//
// This package contains fundamental types like SourceID and ResourceType that are
// referenced by multiple packages (sources, provenance, reconciler, etc.) to avoid
// import cycles while maintaining type safety.
//
// The package has zero dependencies and serves as a foundation for the type system.
//
//nolint:revive // Package name 'types' is appropriate for common type definitions
package types
