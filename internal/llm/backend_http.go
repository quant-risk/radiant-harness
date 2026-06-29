//go:build with_full

package llm

import "context"

// HTTPBackend implements Backend via HTTP for any OpenAI-compatible provider
// (Anthropic native, OpenRouter, OpenAI, xAI, etc.). It is a thin wrapper
// over the existing Client, preserving the retry/backoff/429 logic.
//
// This file is excluded from the Light build (//go:build !with_full) —
// Light binaries have no HTTP LLM layer and cannot talk to providers
// directly. They must rely on the host agent via MCP sampling.
//
// To produce a Light binary: `go build -tags light_only ./cmd/radiant`
// To produce a Full binary:  `go build ./cmd/radiant` (default)
type HTTPBackend struct {
	client *Client
}

// NewHTTPBackend creates an HTTPBackend from a Model configuration. The
// returned backend owns its own *http.Client with the standard timeout.
//
// NOT AVAILABLE in the Light build — see file header.
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

// Compile-time interface conformance check (Full build only).
var _ Backend = (*HTTPBackend)(nil)
