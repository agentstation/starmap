package runtime

import "context"

// OwnedSource supplies a source and its shutdown operation.
// Shutdown cancels and joins source-owned work unless its context ends first.
// With a live context, it must finish joining before returning any cleanup error.
// Repeated calls must be safe.
type OwnedSource interface {
	Source
	Shutdown(context.Context) error
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
