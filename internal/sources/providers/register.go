package providers

// This file centralizes all provider imports for self-registration.
// To add a new provider, just add one import line here.

import (
	// Import all provider implementations for side-effect registration
	_ "github.com/agentstation/starmap/internal/sources/providers/anthropic"
	_ "github.com/agentstation/starmap/internal/sources/providers/cerebras"
	_ "github.com/agentstation/starmap/internal/sources/providers/deepseek"
	_ "github.com/agentstation/starmap/internal/sources/providers/google-ai-studio"
	_ "github.com/agentstation/starmap/internal/sources/providers/google-vertex"
	_ "github.com/agentstation/starmap/internal/sources/providers/groq"
	_ "github.com/agentstation/starmap/internal/sources/providers/openai"
)
