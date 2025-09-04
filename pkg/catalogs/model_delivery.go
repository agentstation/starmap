package catalogs

// ModelDelivery represents technical response delivery capabilities.
type ModelDelivery struct {
	// Response delivery mechanisms
	Protocols []ModelResponseProtocol `json:"protocols,omitempty" yaml:"protocols,omitempty"` // Supported delivery protocols (HTTP, gRPC, etc.)
	Streaming []ModelStreaming        `json:"streaming,omitempty" yaml:"streaming,omitempty"` // Supported streaming modes (sse, websocket, chunked)
	Formats   []ModelResponseFormat   `json:"formats,omitempty" yaml:"formats,omitempty"`     // Available response formats (if format_response feature enabled)
}

// ModelResponseFormat represents a supported response format.
type ModelResponseFormat string

// String returns the string representation of a ModelResponseFormat.
func (mrf ModelResponseFormat) String() string {
	return string(mrf)
}

// Model response formats.
const (
	// Basic formats
	ModelResponseFormatText ModelResponseFormat = "text" // Plain text responses (default)

	// JSON formats
	ModelResponseFormatJSON       ModelResponseFormat = "json"        // JSON encouraged via prompting
	ModelResponseFormatJSONMode   ModelResponseFormat = "json_mode"   // Forced valid JSON (OpenAI style)
	ModelResponseFormatJSONObject ModelResponseFormat = "json_object" // Same as json_mode (OpenAI API name)

	// Structured formats
	ModelResponseFormatJSONSchema       ModelResponseFormat = "json_schema"       // Schema-validated JSON (OpenAI structured output)
	ModelResponseFormatStructuredOutput ModelResponseFormat = "structured_output" // General structured output support

	// Function calling (alternative to JSON schema)
	ModelResponseFormatFunctionCall ModelResponseFormat = "function_call" // Tool/function calling for structured data
)

// ModelResponseProtocol represents a supported delivery protocol.
type ModelResponseProtocol string

// Model delivery protocols.
const (
	ModelResponseProtocolHTTP      ModelResponseProtocol = "http"      // HTTP/HTTPS REST API
	ModelResponseProtocolGRPC      ModelResponseProtocol = "grpc"      // gRPC protocol
	ModelResponseProtocolWebSocket ModelResponseProtocol = "websocket" // WebSocket protocol
)

// ModelStreaming represents how responses can be delivered.
type ModelStreaming string

// String returns the string representation of a ModelStreaming.
func (ms ModelStreaming) String() string {
	return string(ms)
}

// Model streaming modes.
const (
	ModelStreamingSSE       ModelStreaming = "sse"       // Server-Sent Events streaming
	ModelStreamingWebSocket ModelStreaming = "websocket" // WebSocket streaming
	ModelStreamingChunked   ModelStreaming = "chunked"   // HTTP chunked transfer encoding
)
