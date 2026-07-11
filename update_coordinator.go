package starmap

import (
	"context"
	"sync"
)

// updateCoordinator serializes complete catalog update transactions. Its zero
// value is ready for use so test and embedded clients cannot bypass the seam.
type updateCoordinator struct {
	once sync.Once
	slot chan struct{}
}

func (c *updateCoordinator) acquire(ctx context.Context) (func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c.once.Do(func() {
		c.slot = make(chan struct{}, 1)
	})

	select {
	case c.slot <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-c.slot
			return nil, err
		}
		return func() { <-c.slot }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
