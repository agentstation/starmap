package consumer

import (
	"context"
	"testing"
)

func TestPublish(t *testing.T) {
	if err := Publish(context.Background()); err != nil {
		t.Fatalf("Publish: %v", err)
	}
}
