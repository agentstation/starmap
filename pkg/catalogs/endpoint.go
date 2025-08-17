package catalogs

// Endpoint represents an endpoint configuration.
type Endpoint struct {
	ID          string `json:"id" yaml:"id"`                                       // Unique endpoint identifier
	Name        string `json:"name" yaml:"name"`                                   // Display name (must not be empty)
	Description string `json:"description,omitempty" yaml:"description,omitempty"` // Description of the endpoint and its use cases
}
