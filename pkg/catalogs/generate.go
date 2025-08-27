//go:generate gomarkdoc -e -f github -o README.md . --repository.url https://github.com/agentstation/starmap --repository.default-branch master --repository.path /pkg/catalogs

// Package catalogs provides a unified interface for managing AI model catalogs
// with multiple storage backends.
package catalogs