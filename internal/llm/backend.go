package llm

import "context"

// Backend is the abstraction the loop uses for all LLM calls. It decouples
// the runner from the transport — HTTP (Anthropic, OpenRouter, OpenAI) in the
// normal case, or MCP sampling/createMessage over a JSON-RPC pipe when the
// harness is driven by a host agent that provides inference.
//
// Implementations:
//   - HTTPBackend: thin wrapper over the existing Client.
//   - SamplingBackend: MCP sampling/createMessage via stdin/stdout.
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

// HTTPBackend implements Backend via HTTP for any OpenAI-compatible provider
// (Anthropic native, OpenRouter, OpenAI, xAI, etc.). It is a thin wrapper
// over the existing Client, preserving the retry/backoff/429 logic.
type HTTPBackend struct {
	client *Client
}

// NewHTTPBackend creates an HTTPBackend from a Model configuration. The
// returned backend owns its own *http.Client with the standard timeout.
func NewHTTPBackend(m Model) *HTTPBackend {
	return &HTTPBackend{client: NewClient(m)}
}

// Chat delegates to the underlying Client.Chat.
func (b *HTTPBackend) Chat(ctx context.Context, msgs []Message) (*ChatResponse, error) {
	return b.client.Chat(ctx, msgs)
}

// ChatStream delegates to the underlying Client.ChatStream.
func (b *HTTPBackend) ChatStream(ctx context.Context, msgs []Message, cb StreamCallback) (*ChatResponse, error) {
	return b.client.ChatStream(ctx, msgs, cb)
}

// ModelID returns the configured model identifier.
func (b *HTTPBackend) ModelID() string { return b.client.model.Model }

// Compile-time interface conformance checks.
var (
	_ Backend = (*HTTPBackend)(nil)
	_ Backend = (*SamplingBackend)(nil)
)
