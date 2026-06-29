package llm

import "context"

// Backend is the abstraction the loop uses for all LLM calls. It decouples
// the runner from the transport. The only transport bundled with radiant is
// MCP sampling/createMessage (SamplingBackend): every LLM call is routed back
// to the host agent, which pays for inference and is responsible for the
// model choice.
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

// Compile-time interface conformance check.
var _ Backend = (*SamplingBackend)(nil)
