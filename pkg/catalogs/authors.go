package catalogs

import (
	"maps"
	"sort"
	"sync"

	"github.com/agentstation/starmap/pkg/errors"
)

// Authors is a concurrent safe map of authors.
type Authors struct {
	mu      sync.RWMutex
	authors map[AuthorID]*Author
}

// AuthorsOption defines a function that configures an Authors instance.
type AuthorsOption func(*Authors)

// WithAuthorsCapacity sets the initial capacity of the authors map.
func WithAuthorsCapacity(capacity int) AuthorsOption {
	return func(a *Authors) {
		a.authors = make(map[AuthorID]*Author, capacity)
	}
}

// WithAuthorsMap initializes the map with existing authors.
func WithAuthorsMap(authors map[AuthorID]*Author) AuthorsOption {
	return func(a *Authors) {
		if authors != nil {
			a.authors = make(map[AuthorID]*Author, len(authors))
			maps.Copy(a.authors, authors)
		}
	}
}

// NewAuthors creates a new Authors map with optional configuration.
func NewAuthors(opts ...AuthorsOption) *Authors {
	a := &Authors{
		authors: make(map[AuthorID]*Author),
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// Get returns an author by id and whether it exists.
func (a *Authors) Get(id AuthorID) (*Author, bool) {
	a.mu.RLock()
	author, ok := a.authors[id]
	a.mu.RUnlock()
	return author, ok
}

// Set sets an author by id. Returns an error if author is nil.
func (a *Authors) Set(id AuthorID, author *Author) error {
	if author == nil {
		return &errors.ValidationError{
			Field:   "author",
			Message: "cannot be nil",
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.authors[id] = author
	return nil
}

// Add adds an author, returning an error if it already exists.
func (a *Authors) Add(author *Author) error {
	if author == nil {
		return &errors.ValidationError{
			Field:   "author",
			Message: "cannot be nil",
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.authors[author.ID]; exists {
		return &errors.ValidationError{
			Field:   "author.ID",
			Value:   author.ID,
			Message: "already exists",
		}
	}

	a.authors[author.ID] = author
	return nil
}

// Delete removes an author by id. Returns an error if the author doesn't exist.
func (a *Authors) Delete(id AuthorID) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.authors[id]; !exists {
		return &errors.NotFoundError{
			Resource: "author",
			ID:       string(id),
		}
	}

	delete(a.authors, id)
	return nil
}

// Exists checks if an author exists without returning it.
func (a *Authors) Exists(id AuthorID) bool {
	a.mu.RLock()
	_, exists := a.authors[id]
	a.mu.RUnlock()
	return exists
}

// Len returns the number of authors.
func (a *Authors) Len() int {
	a.mu.RLock()
	length := len(a.authors)
	a.mu.RUnlock()
	return length
}

// List returns a slice of all authors as values (copies).
func (a *Authors) List() []Author {
	a.mu.RLock()
	authors := make([]Author, 0, len(a.authors))
	for _, author := range a.authors {
		// Return deep copies to prevent external modification
		authors = append(authors, DeepCopyAuthor(*author))
	}
	a.mu.RUnlock()

	// Sort by ID for deterministic ordering
	sort.Slice(authors, func(i, j int) bool {
		return authors[i].ID < authors[j].ID
	})

	return authors
}

// Map returns a copy of all authors.
func (a *Authors) Map() map[AuthorID]*Author {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[AuthorID]*Author, len(a.authors))
	maps.Copy(result, a.authors)
	return result
}

// ForEach applies a function to each author. The function should not modify the author.
// If the function returns false, iteration stops early.
func (a *Authors) ForEach(fn func(id AuthorID, author *Author) bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for id, author := range a.authors {
		if !fn(id, author) {
			break
		}
	}
}

// Clear removes all authors.
func (a *Authors) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	// Clear existing map instead of allocating new one
	for k := range a.authors {
		delete(a.authors, k)
	}
}

// AddBatch adds multiple authors in a single operation.
// Only adds authors that don't already exist - fails if an author ID already exists.
// Returns a map of author IDs to errors for any failed additions.
func (a *Authors) AddBatch(authors []*Author) map[AuthorID]error {
	if len(authors) == 0 {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	errs := make(map[AuthorID]error)

	// First pass: validate all authors
	for _, author := range authors {
		if author == nil {
			continue // Skip nil authors
		}
		if _, exists := a.authors[author.ID]; exists {
			errs[author.ID] = &errors.ValidationError{
				Field:   "author.ID",
				Value:   author.ID,
				Message: "already exists",
			}
		}
	}

	// Second pass: add valid authors
	for _, author := range authors {
		if author == nil {
			continue
		}
		if _, hasError := errs[author.ID]; !hasError {
			a.authors[author.ID] = author
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}

// SetBatch sets multiple authors in a single operation.
// Overwrites existing authors or adds new ones (upsert behavior).
// Returns an error if any author is nil.
func (a *Authors) SetBatch(authors map[AuthorID]*Author) error {
	if len(authors) == 0 {
		return nil
	}

	// Validate all authors first
	for id, author := range authors {
		if author == nil {
			return &errors.ValidationError{
				Field:   "authors[" + string(id) + "]",
				Message: "cannot be nil",
			}
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for id, author := range authors {
		a.authors[id] = author
	}

	return nil
}

// DeleteBatch removes multiple authors by their IDs.
// Returns a map of errors for authors that couldn't be deleted (not found).
func (a *Authors) DeleteBatch(ids []AuthorID) map[AuthorID]error {
	if len(ids) == 0 {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	errs := make(map[AuthorID]error)
	for _, id := range ids {
		if _, exists := a.authors[id]; !exists {
			errs[id] = &errors.NotFoundError{
				Resource: "author",
				ID:       string(id),
			}
		} else {
			delete(a.authors, id)
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errs
}
