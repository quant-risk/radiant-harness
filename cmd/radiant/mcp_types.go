package main

// This file holds the MCP (Model Context Protocol) server types.
// They were previously inlined in cmd/radiant/helpers.go (a 4931-line
// god file). Splitting them out makes the MCP surface auditable —
// any change to the JSON-RPC shapes is now in one file, and security
// review can read this in isolation.
//
// Wire shapes follow MCP spec §6 (JSON-RPC 2.0). The host agent
// (Claude Code, Hermes, Cursor, Codex) sends requests on stdin and
// reads responses from stdout. See internal/llm/sampling.go for the
// sampling/createMessage subprotocol that runs on top of this.

import (
	"encoding/json"

	"github.com/quant-risk/radiant-harness/v3/internal/llm"
)

// mcpTool is one tool exposed by the MCP server.
type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema mcpInputSchema `json:"inputSchema"`
}

// mcpInputSchema is the JSON Schema describing the tool's args.
type mcpInputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]mcpPropertyDef `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

// mcpPropertyDef is one property in the input schema.
type mcpPropertyDef struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// mcpRequest is a JSON-RPC 2.0 request from the client.
type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// mcpResponse is a JSON-RPC 2.0 response to the client.
type mcpResponse struct {
	JSONRPC  string      `json:"jsonrpc"`
	ID       any         `json:"id,omitempty"`
	Result   interface{} `json:"result,omitempty"`
	Error    *mcpError   `json:"error,omitempty"`
	// suppress=true means "do not encode this — it answers a notification
	// (JSON-RPC `id` absent) and per the spec MUST NOT elicit a reply."
	// Only used internally by the read loop in cmd_mcp_runtime.go.
	suppress bool `json:"-"`
}

// mcpError is the error block of a JSON-RPC response.
type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// mcpDispatcher carries the execution context for a running MCP server session.
// It is created once per runMCPServe call and threaded explicitly through the
// dispatch chain, replacing the former package-level activeSamplingBackend global.
type mcpDispatcher struct {
	// sampling is non-nil only when --sampling mode is active.
	// When non-nil, radiant_possess routes all LLM calls through it instead
	// of the HTTP API. The host agent provides inference via
	// sampling/createMessage responses delivered to Dispatch().
	sampling *llm.SamplingBackend
}

// backend returns the llm.Backend to use for radiant_possess calls, or nil
// in non-sampling mode (caller uses the HTTP path).
func (d *mcpDispatcher) backend() llm.Backend {
	if d == nil || d.sampling == nil {
		return nil
	}
	return d.sampling
}