package llm

import "context"

// Backend is the abstraction the loop uses for all LLM calls. It decouples
// the runner from the transport — HTTP (Anthropic, OpenRouter, OpenAI) in the
// Full build, or MCP sampling/createMessage over a JSON-RPC pipe in both
// Light and Full builds when the harness is driven by a host agent that
// provides inference.
//
// Light build: only SamplingBackend satisfies this interface (HTTP files
// are excluded via build tags). Inference comes from the host agent.
//
// Full build (default): both SamplingBackend and HTTPBackend are
// available. Inference source is selected by PickBackend based on runtime
// context.
type Backend interface {
	// Chat sends messages and returns a synchronous completion.
	Chat(ctx context.Context, messages []Message) (*ChatResponse, error)

	// ChatStream sends messages and streams the response via callback.
	// If streaming is not supported, implementations may delegate to Chat
	// and invoke the callback once with the full text.
	ChatStream(ctx context.Context, messages []Message, cb StreamCallback) (*ChatResponse, error)

	// ModelID returns the identifier of the model backing this Backend.
	// Used for cost estimation and trace logging.
	ModelID() string
}

// Compile-time interface conformance check for the always-available backend.
// The HTTPBackend conformance check lives in backend_http.go (only compiled
// in the Full build).
var _ Backend = (*SamplingBackend)(nil)
