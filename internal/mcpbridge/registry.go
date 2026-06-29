package mcpbridge

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/quant-risk/radiant-harness/internal/tools"
)

// MCPTool is one tool advertised by an MCP server. Matches the
// shape returned by `tools/list` in the MCP spec.
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ListTools returns the tools advertised by the server.
func (c *Client) ListTools(ctx context.Context) ([]MCPTool, error) {
	callCtx, cancel := context.WithTimeout(ctx, listToolsTimeout)
	defer cancel()

	result, err := c.call(callCtx, "tools/list", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("tools/list: %w", err)
	}
	var resp struct {
		Tools []MCPTool `json:"tools"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("tools/list decode: %w", err)
	}
	return resp.Tools, nil
}

// CallTool invokes a tool by name with the given arguments. The
// raw response payload is returned — usually a JSON object with
// `content` (a list of typed content blocks) and `isError`.
//
// The caller is responsible for interpreting the content blocks;
// the bridge doesn't try to flatten them into a single string,
// because LLM-facing tools may produce structured outputs that
// need to be passed through.
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	callCtx, cancel := context.WithTimeout(ctx, callToolTimeout)
	defer cancel()

	params := map[string]any{
		"name":      name,
		"arguments": args,
	}
	result, err := c.call(callCtx, "tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("tools/call %s: %w", name, err)
	}

	// Check isError flag — MCP servers report tool-level errors
	// via isError=true with the error message in the content blocks.
	var wrapper struct {
		IsError bool            `json:"isError"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(result, &wrapper); err == nil && wrapper.IsError {
		return result, fmt.Errorf("mcp_bridge: tool %s returned isError=true", name)
	}
	return result, nil
}

// ToLocalTool converts an MCP tool into a tools.Tool bound to this
// client. The Invoke function dispatches through Client.CallTool.
//
// The MCP server is the source of truth for the schema, but the
// local tools.Registry uses a simpler Param slice. JSON Schema
// properties are flattened — type, description, and required are
// preserved where possible. Nested schemas (oneOf, $ref, etc.) are
// passed through as a single opaque Param so the LLM still sees
// the raw structure.
func (m MCPTool) ToLocalTool(client *Client) *tools.Tool {
	params := flattenSchema(m.InputSchema)
	return &tools.Tool{
		Name:        client.Name() + "__" + m.Name,
		Description: m.Description + " (bridged from MCP server: " + client.Name() + ")",
		Params:      params,
		Invoke: func(ctx context.Context, args json.RawMessage) (any, error) {
			return client.CallTool(ctx, m.Name, args)
		},
	}
}

// flattenSchema parses an MCP JSON Schema (inputSchema) into a
// tools.Param slice. Top-level only — nested objects are passed
// through as a single opaque Param with type "object" so the LLM
// sees the raw schema in the description.
func flattenSchema(schema json.RawMessage) []tools.Param {
	if len(schema) == 0 {
		return nil
	}
	var parsed struct {
		Type       string                    `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                  `json:"required"`
	}
	if err := json.Unmarshal(schema, &parsed); err != nil {
		// Couldn't parse — pass through as opaque.
		return []tools.Param{{
			Name:        "input",
			Type:        "object",
			Description: "raw MCP schema: " + string(schema),
			Required:    true,
		}}
	}
	if parsed.Type != "object" {
		// Not an object schema — pass through as opaque.
		return []tools.Param{{
			Name:        "input",
			Type:        "object",
			Description: "raw MCP schema: " + string(schema),
			Required:    true,
		}}
	}

	required := map[string]bool{}
	for _, r := range parsed.Required {
		required[r] = true
	}

	var out []tools.Param
	for name, propSchema := range parsed.Properties {
		p := tools.Param{
			Name:     name,
			Required: required[name],
		}
		var prop struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(propSchema, &prop); err == nil {
			p.Type = prop.Type
			p.Description = prop.Description
		}
		if p.Type == "" {
			p.Type = "string" // safe fallback
		}
		out = append(out, p)
	}
	return out
}