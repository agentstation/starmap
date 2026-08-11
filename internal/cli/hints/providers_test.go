package hints

import (
	"strings"
	"testing"
)

func TestAuthenticationHintContainsNoProviderCredentialFact(t *testing.T) {
	hints := authHintProvider(Context{
		Command: authCommand, Subcommand: "status",
		UserState: UserState{AuthProviders: nil},
	})
	if len(hints) != 1 {
		t.Fatalf("hint count = %d, want 1", len(hints))
	}
	text := hints[0].Message + "\n" + hints[0].Command
	for _, prohibited := range []string{"OPENAI_API_KEY", "openai", "anthropic", "groq"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(prohibited)) {
			t.Fatalf("hint contains provider fact %q: %s", prohibited, text)
		}
	}
	if strings.Contains(text, "providers auth") {
		t.Fatalf("hint contains removed providers auth command: %s", text)
	}
	if hints[0].Command != "starmap providers" {
		t.Fatalf("hint command = %q, want %q", hints[0].Command, "starmap providers")
	}
}
