package serve

import "testing"

func TestCORSOriginAllowlistEnablesCORS(t *testing.T) {
	command := NewCommand(nil)
	if err := command.Flags().Set(
		"cors-origins",
		"https://gateway.example.test",
	); err != nil {
		t.Fatalf("set cors-origins: %v", err)
	}

	config := parseConfig(command)
	if !config.CORSEnabled {
		t.Fatal("explicit CORS origin allowlist did not enable CORS")
	}
	if len(config.CORSOrigins) != 1 ||
		config.CORSOrigins[0] != "https://gateway.example.test" {
		t.Fatalf("CORS origins = %v", config.CORSOrigins)
	}
}
