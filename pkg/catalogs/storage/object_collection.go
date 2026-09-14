package storage

import (
	"context"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

// MaxObjectListEntries bounds one object inventory page.
const MaxObjectListEntries = 1000

// ObjectListRequest selects one bounded page beneath a nonempty prefix.
// Cursor is opaque. Reuse a returned cursor with the same prefix.
type ObjectListRequest struct {
	Prefix string
	Cursor string
	Limit  int
}

// Validate rejects unbounded inventories and empty namespaces.
func (r ObjectListRequest) Validate() error {
	if strings.TrimSpace(r.Prefix) == "" {
		return &errors.ValidationError{Field: "object.list.prefix", Message: "a nonempty prefix is required"}
	}
	if r.Limit < 1 || r.Limit > MaxObjectListEntries {
		return &errors.ValidationError{Field: "object.list.limit", Message: "must be between 1 and 1000"}
	}
	return nil
}

// ObjectEntry describes one current object without reading its payload.
// Version is its conditional validator, not a monotonic fencing token.
type ObjectEntry struct {
	Key     string
	Version string
	Size    int64
}

// ObjectPage contains at most the requested number of objects.
// A nonempty Next cursor requires another page.
type ObjectPage struct {
	Objects []ObjectEntry
	Next    string
}

// ObjectCollectionBackend adds inventory and conditional deletion to ObjectBackend.
// Pages need not form a snapshot across calls. The caller must coordinate
// publication and retention before deleting an object. These operations alone
// do not protect generations, pins, or readers.
//
// Delete requires an exact nonempty validator. A missing object can return
// success, a typed not-found error, or a conditional conflict.
// Versioned backends can retain historical versions after current-object deletion.
type ObjectCollectionBackend interface {
	ObjectBackend
	List(context.Context, ObjectListRequest) (ObjectPage, error)
	Delete(context.Context, string, string) error
}
