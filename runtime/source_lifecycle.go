package runtime

// OwnedSource supplies a source and its shutdown operation.
// Close must cancel and join source-owned work. Repeated calls must be safe.
type OwnedSource interface {
	Source
	Close() error
}

// WithOwnedSource transfers the selected source to the runtime during Open.
// Open failure and runtime shutdown close it. Only the final selected source transfers.
// WithSource keeps ownership with the caller and replaces this selection.
func WithOwnedSource(source OwnedSource) Option {
	return func(config *options) error {
		if err := WithSource(source)(config); err != nil {
			return err
		}
		config.ownedSource = source
		return nil
	}
}
