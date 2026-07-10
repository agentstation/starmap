// Package catalogmeta provides shared catalog metadata definitions used across
// Starmap packages.
//
// This package contains fundamental types like SourceID and ResourceType that are
// referenced by multiple packages (sources, provenance, reconciler, etc.) to avoid
// import cycles while maintaining type safety.
//
// The package has zero dependencies and serves as a foundation for the type system.
package catalogmeta
