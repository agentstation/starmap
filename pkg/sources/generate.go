//go:generate gomarkdoc -e -f github -o README.md . --repository.url https://github.com/agentstation/starmap --repository.default-branch master --repository.path /pkg/sources

// Package sources provides abstractions for fetching AI model catalog data
// from various external sources including provider APIs and community repositories.
package sources