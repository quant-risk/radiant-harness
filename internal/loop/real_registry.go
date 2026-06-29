package loop

import (
	"context"
	"fmt"

	"github.com/quant-risk/radiant-harness/internal/mcpbridge"
	"github.com/quant-risk/radiant-harness/internal/tools"
	"github.com/quant-risk/radiant-harness/internal/tools/fs"
	"github.com/quant-risk/radiant-harness/internal/tools/gate"
)

// MCPSpec is one MCP bridge to register at startup. The CLI builds
// a slice of these from `--mcp-bridge "name:command args"` flags.
type MCPSpec struct {
	Name    string   // short identifier (e.g. "github", "fs")
	Command string   // executable path
	Args    []string // command args
}

// RealRegistry returns the registry with the concrete tools available
// in the current release. Callers wire this into Engine.ToolRegistry
// to enable structured tool-use; the dispatcher in internal/engine
// then routes ```tool_call``` fences from LLM output through the
// registry instead of the legacy code-block emission path.
//
// Sprint 69 (v2.38.0): write_file (atomic write, fsutil.PathIsSafe).
// Sprint 70 (v2.39.0): + read_file, search_code.
// Sprint 71 (v2.40.0): + run_gate (gaterun wrapper + policy allowlist).
// Sprint 72 (v2.41.0): + MCP bridge (external server tools).
//
// The projectDir is captured by concrete tools (like fs.WriteFileTool)
// that need it for boundary checks. Pass the project root — the same
// value used to initialise the Engine.
//
// Pass `mcpBridges` to register external MCP servers' tools; nil
// means only the built-in tools. Failures to dial an MCP server
// are surfaced via the returned error so the CLI can decide whether
// to abort or continue.
//
// This function lives in internal/loop (not internal/tools) to break
// an import cycle: tools/fs imports tools (for Tool/Param types),
// and tools/Default imports fs to expose the real registry. Putting
// RealRegistry here lets both internal/tools (via the var indirection
// in tools.go) and the CLI / Engine callers reach it without the
// cycle.
func RealRegistry(projectDir string, mcpBridges ...MCPSpec) (*tools.Registry, error) {
	r := tools.NewRegistry()
	r.Register(fs.WriteFileTool(projectDir))
	r.Register(fs.ReadFileTool(projectDir))
	r.Register(fs.SearchCodeTool(projectDir))
	r.Register(gate.RunGateTool(projectDir))

	for _, spec := range mcpBridges {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel() // ensure cleanup if mid-iteration dial fails

		_, tools, err := mcpbridge.LoadTools(ctx, spec.Name, spec.Command, spec.Args)
		if err != nil {
			return nil, fmt.Errorf("mcp_bridge %s: %w", spec.Name, err)
		}
		for _, t := range tools {
			t := t // pin
			r.Register(&t)
		}
	}
	return r, nil
}

// RealRegistrySimple returns the registry without any MCP bridges.
// Convenience wrapper around RealRegistry for callers that don't
// need MCP integration.
func RealRegistrySimple(projectDir string) *tools.Registry {
	r, err := RealRegistry(projectDir)
	if err != nil {
		// Built-in tools never fail to register; an error here
		// means a programming bug. Return a partial registry so
		// the program can still run; the caller can log the error.
		return r
	}
	return r
}

// init wires the SimpleRealRegistry implementation into the indirection
// declared in internal/tools. The tools package exposes
// RealRegistry as a thin re-export; this init replaces the no-op
// placeholder with the concrete builder.
func init() {
	tools.SetRealRegistryBuilder(func(projectDir string) *tools.Registry {
		return RealRegistrySimple(projectDir)
	})
}