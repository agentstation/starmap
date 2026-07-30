package consumer

import (
	"context"
	"testing"
)

func TestActivatePinned(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")

	if err := ActivatePinned(context.Background()); err != nil {
		t.Fatalf("ActivatePinned: %v", err)
	}
}
