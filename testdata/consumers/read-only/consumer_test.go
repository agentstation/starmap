package consumer

import "testing"

func TestLookup(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")

	if err := Lookup(); err != nil {
		t.Fatalf("Lookup: %v", err)
	}
}
