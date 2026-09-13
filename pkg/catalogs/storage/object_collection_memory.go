package storage

import (
	"context"
	"slices"
	"strings"

	"github.com/agentstation/starmap/pkg/errors"
)

// List returns one ordered page of current objects beneath the requested prefix.
func (b *MemoryObjectBackend) List(ctx context.Context, request ObjectListRequest) (ObjectPage, error) {
	if err := request.Validate(); err != nil {
		return ObjectPage{}, err
	}
	if request.Cursor != "" && !strings.HasPrefix(request.Cursor, request.Prefix) {
		return ObjectPage{}, &errors.ValidationError{Field: "object.list.cursor", Message: "does not belong to the requested prefix"}
	}
	if err := ctx.Err(); err != nil {
		return ObjectPage{}, err
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		return ObjectPage{}, err
	}
	// Keep only the smallest page and one continuation witness in memory.
	keys := make([]string, 0, request.Limit+1)
	for key := range b.objects {
		if err := ctx.Err(); err != nil {
			return ObjectPage{}, err
		}
		if !strings.HasPrefix(key, request.Prefix) || key <= request.Cursor {
			continue
		}
		index, _ := slices.BinarySearch(keys, key)
		if index > request.Limit {
			continue
		}
		if len(keys) <= request.Limit {
			keys = append(keys, "")
		}
		copy(keys[index+1:], keys[index:len(keys)-1])
		keys[index] = key
	}
	page := ObjectPage{}
	if len(keys) > request.Limit {
		keys = keys[:request.Limit]
		page.Next = keys[len(keys)-1]
	}
	for _, key := range keys {
		value := b.objects[key]
		page.Objects = append(page.Objects, ObjectEntry{Key: key, Version: value.Version, Size: int64(len(value.Data))})
	}
	return page, nil
}

// Delete removes an object only when its current validator matches version.
func (b *MemoryObjectBackend) Delete(ctx context.Context, key, version string) error {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(version) == "" || version == "*" {
		return &errors.ValidationError{Field: "object.delete", Message: "key and exact version are required"}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	value, found := b.objects[key]
	if !found {
		return &errors.NotFoundError{Resource: "object", ID: key}
	}
	if value.Version != version {
		return &errors.ConflictError{Resource: "object", Expected: version, Actual: value.Version}
	}
	delete(b.objects, key)
	return nil
}

var _ ObjectCollectionBackend = (*MemoryObjectBackend)(nil)
