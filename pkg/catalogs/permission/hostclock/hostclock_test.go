package hostclock

import (
	"context"
	"errors"
	"testing"
)

func TestObserveCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	reading, err := Observe(ctx)
	if !errors.Is(err, context.Canceled) || reading.Known {
		t.Fatalf("canceled observation = %+v, %v", reading, err)
	}
}

func TestObserveRequiresContext(t *testing.T) {
	reading, err := Observe(nil)
	if err == nil || reading.Known {
		t.Fatalf("observation without context = %+v, %v", reading, err)
	}
}
