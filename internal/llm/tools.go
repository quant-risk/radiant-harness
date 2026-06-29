// Package llm — agentic tool-calling contract (v3.7.0+).
//
// SamplingBackend and HTTPBackend can OPTIONALLY implement ChatWithTools
// to participate in the agentic loop (radiant_possess drives file I/O,
// shell commands, etc. through the model). Backends that don't
// implement it fall back to text-only Chat(); the driver detects this
// via type assertion and routes accordingly.
//
// This is intentionally NOT on the Backend interface so existing call
// sites that build a Backend from a text-only path are not broken.
package llm

import "context"

// ToolCapable is the optional capability surface a Backend implements
// to support native tool-use round-trips. A driver checks via the
// standard type-assertion:
//
//	if tc, ok := backend.(llm.ToolCapable); ok {
//	    resp, err = tc.ChatWithTools(ctx, messages, tools, choice)
//	}
//
// When the assertion fails the driver either falls back to text-only
// Chat() or downgrades to the self-driven scaffold path (whichever
// the calling surface prefers).
//
// Concurrency: implementations may be called concurrently from
// multiple drivers. The SamplingBackend in particular serialises
// requests through a single JSON-RPC channel — see
// internal/llm/sampling.go::SetWriteMu.
type ToolCapable interface {
	// ChatWithTools sends messages alongside a set of offered tools
	// and returns a completion that may include native tool_calls.
	//
	// `tools` may be nil/empty — implementations should still return
	// a normal text response in that case (equivalent to Chat()).
	//
	// `choice` is optional. Pass nil to let the backend pick the
	// default (auto).
	ChatWithTools(ctx context.Context, messages []Message, tools []Tool, choice *ToolChoice) (*ChatResponse, error)
}
