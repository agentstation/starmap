// Package consumer is a real external module exercising the public reactive
// Starmap subscriber without importing CLI or internal packages.
package consumer

import (
	"context"
	"errors"
	"fmt"

	"github.com/agentstation/starmap/remote"
)

// VerifyRemoteCatalog starts, reads, and joins one caller-owned reactive
// subscriber lifecycle.
func VerifyRemoteCatalog(ctx context.Context, baseURL string) (err error) {
	subscriber, err := remote.New(remote.Config{BaseURL: baseURL})
	if err != nil {
		return err
	}
	if err := subscriber.Start(ctx); err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, subscriber.Close())
	}()

	catalog := subscriber.Catalog()
	if catalog == nil {
		return fmt.Errorf("remote catalog is nil")
	}
	if _, err := catalog.FindModel("gpt-4o"); err != nil {
		return fmt.Errorf("find remote model: %w", err)
	}
	return nil
}
