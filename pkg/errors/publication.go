package errors

import "fmt"

// PublicationError reports visible publication whose durability remains unconfirmed.
// Resource and ID identify the published record. Callers must not infer rollback.
// Retry the same content and identity after resolving the underlying failure.
type PublicationError struct {
	Resource string
	ID       string
	Err      error
}

// Error identifies the publication and its durability failure.
func (e *PublicationError) Error() string {
	return fmt.Sprintf("%s %q is published but durability is unconfirmed: %v", e.Resource, e.ID, e.Err)
}

// Unwrap preserves the underlying filesystem error.
func (e *PublicationError) Unwrap() error {
	return e.Err
}
