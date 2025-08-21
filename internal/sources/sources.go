package sources

// Import all source implementations for auto-registration via init()
import (
	_ "github.com/agentstation/starmap/internal/sources/local"
	_ "github.com/agentstation/starmap/internal/sources/modelsdev"
	_ "github.com/agentstation/starmap/internal/sources/provider"
)