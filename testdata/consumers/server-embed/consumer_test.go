package consumer

import (
	"context"
	"testing"
	"time"
)

func TestServeAndShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := ServeAndShutdown(ctx); err != nil {
		t.Fatalf("ServeAndShutdown: %v", err)
	}
}
