package loop

import (
	"github.com/quant-risk/radiant-harness/internal/tools"
	"github.com/quant-risk/radiant-harness/internal/tools/fs"
	"github.com/quant-risk/radiant-harness/internal/tools/gate"
)

// RealRegistry returns the registry with the concrete tools available
// in the current release. Callers wire this into Engine.ToolRegistry
// to enable structured tool-use; the dispatcher in internal/engine
// then routes ```tool_call``` fences from LLM output through the
// registry instead of the legacy code-block emission path.
//
// Sprint 69 (v2.38.0): write_file (atomic write, fsutil.PathIsSafe).
// Sprint 70 (v2.39.0): + read_file, search_code.
// Sprint 71 (v2.40.0): + run_gate (gaterun wrapper + policy allowlist).
//
// The projectDir is captured by concrete tools (like fs.WriteFileTool)
// that need it for boundary checks. Pass the project root — the same
// value used to initialise the Engine.
//
// This function lives in internal/loop (not internal/tools) to break
// an import cycle: tools/fs imports tools (for Tool/Param types),
// and tools/Default imports fs to expose the real registry. Putting
// RealRegistry here lets both internal/tools (via the var indirection
// in tools.go) and the CLI / Engine callers reach it without the
// cycle.
func RealRegistry(projectDir string) *tools.Registry {
	r := tools.NewRegistry()
	r.Register(fs.WriteFileTool(projectDir))
	r.Register(fs.ReadFileTool(projectDir))
	r.Register(fs.SearchCodeTool(projectDir))
	r.Register(gate.RunGateTool(projectDir))
	return r
}

// init wires the RealRegistry implementation into the indirection
// declared in internal/tools. The tools package exposes
// RealRegistry as a thin re-export; this init replaces the no-op
// placeholder with the concrete builder.
func init() {
	tools.SetRealRegistryBuilder(RealRegistry)
}