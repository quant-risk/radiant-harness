// Package tools provides a structured tool-use registry for the
// radiant-harness executor.
//
// Status (v2.37.0): SCAFFOLD ONLY. The executor still relies on
// code-block emission (markdown fences with `// File:` headers) as
// parsed by internal/engine.extractCodeBlocks. This package
// establishes the schema, registry, and invocation interface so
// that follow-up releases can wire tool calls through it without
// another architectural reshuffle.
//
// The goal is to replace the "LLM emits code blocks" pattern with
// "LLM emits structured tool calls" — which gives us:
//
//   1. Validation of arguments at the boundary (no path injection
//      through a malformed code-block path)
//   2. Distinct tracing per tool invocation
//   3. Retry on tool-specific failures (e.g. file not found)
//   4. Pluggable tools — adding a new capability is a one-line
//      registry entry, not a new branch in the executor
//
// MCP alignment: the Tool type mirrors MCP's tool definition
// (name, description, inputSchema). A future "MCP tool bridge"
// adapter can register an MCP server's tools directly into the
// Registry without code changes.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Param is one named parameter in a tool's input schema.
// Type follows JSON Schema: string, number, integer, boolean,
// object, array, or null.
type Param struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required,omitempty"`
}

// Tool is a single named capability exposed to the executor.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Params      []Param `json:"params"`

	// Invoke runs the tool. ctx carries cancellation and the caller's
	// identity (projectDir etc. attached via WithValue). args is the
	// raw JSON the LLM emitted; the implementation is responsible for
	// parsing it according to the schema declared above.
	//
	// Returns a result struct (any JSON-serializable value) and an
	// error. Non-nil error is treated as a recoverable failure —
	// the executor will surface it to the verifier and (per the
	// existing semantics) retry the iteration.
	//
	// A tool can surface structured trace metadata by returning a
	// value that satisfies an `Annotate() map[string]any` method
	// (the engine type-switches against this duck-typed interface).
	// Adding a new tool that wants trace visibility is a one-method
	// change — no engine edits required.
	Invoke func(ctx context.Context, args json.RawMessage) (any, error)
}

// Registry holds the known tools. Built once at startup, then read-only.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*Tool
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]*Tool)}
}

// Register adds a tool to the registry. Panics on duplicate name —
// duplicate registration is a programming error, not a runtime one.
func (r *Registry) Register(t *Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[t.Name]; exists {
		panic("tools: duplicate registration of " + t.Name)
	}
	r.tools[t.Name] = t
}

// Get returns the tool with the given name, or nil.
func (r *Registry) Get(name string) *Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// Names returns the registered tool names in undefined order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.tools))
	for name := range r.tools {
		out = append(out, name)
	}
	return out
}

// Call dispatches a tool invocation by name.
func (r *Registry) Call(ctx context.Context, name string, args json.RawMessage) (any, error) {
	t := r.Get(name)
	if t == nil {
		return nil, fmt.Errorf("tools: unknown tool %q", name)
	}
	if t.Invoke == nil {
		return nil, fmt.Errorf("tools: tool %q has no Invoke", name)
	}
	return t.Invoke(ctx, args)
}

// Default returns the registry preloaded with the built-in tools
// radiant-harness always ships. Callers can add more via Register.
//
// Built-ins are intentionally minimal at v2.37.0 — they validate
// the registry mechanism rather than replace the executor's
// code-block parsing. Each Invoke returns a clear "not wired in
// this release" error so the executor knows to fall back to the
// code-block path.
func Default() *Registry {
	r := NewRegistry()
	r.Register(&Tool{
		Name:        "read_file",
		Description: "Read the contents of a file at the given path. Path must be inside the project directory.",
		Params: []Param{
			{Name: "path", Type: "string", Required: true, Description: "Absolute or project-relative path."},
		},
		Invoke: stubInvoke("read_file"),
	})
	r.Register(&Tool{
		Name:        "write_file",
		Description: "Write content to a file at the given path. Creates parent directories as needed. Path must be inside the project directory.",
		Params: []Param{
			{Name: "path", Type: "string", Required: true, Description: "Path to write to."},
			{Name: "content", Type: "string", Required: true, Description: "complete file contents."},
		},
		Invoke: stubInvoke("write_file"),
	})
	r.Register(&Tool{
		Name:        "search_code",
		Description: "Search the project for a regex pattern. Returns matching lines with file:line:column format.",
		Params: []Param{
			{Name: "pattern", Type: "string", Required: true, Description: "Regex pattern (Go regexp syntax)."},
			{Name: "path", Type: "string", Description: "Directory to search in. Defaults to project root."},
		},
		Invoke: stubInvoke("search_code"),
	})
	r.Register(&Tool{
		Name:        "run_gate",
		Description: "Run a quality gate command (go test, go vet, etc.). Returns stdout/stderr and exit code.",
		Params: []Param{
			{Name: "command", Type: "string", Required: true, Description: "Gate command (must be in the allowlist)."},
		},
		Invoke: stubInvoke("run_gate"),
	})
	return r
}

// stubInvoke returns an Invoke function that always errors with a
// "not wired" message. This lets the registry advertise the surface
// area without making false promises — the executor knows to fall
// back to the existing code-block emission path.
func stubInvoke(name string) func(context.Context, json.RawMessage) (any, error) {
	return func(ctx context.Context, args json.RawMessage) (any, error) {
		return nil, fmt.Errorf("tools: %q is registered but not yet wired into the executor; the code-block emission path is the active one in v2.37.0", name)
	}
}

// realRegistry is the indirection through which internal/loop
// (which can import both this package and internal/tools/fs without
// a cycle) wires the concrete RealRegistry implementation. The
// internal/tools package exposes RealRegistry as a thin re-export
// so callers that already depend on it don't need to add internal/loop
// to their imports just to build a real registry.
//
// The default value returns nil — callers must SetRealRegistryBuilder
// before calling RealRegistry, or pass nil through to Engine.ToolRegistry
// (which then uses the legacy code-block path).
var realRegistry = func(string) *Registry { return nil }

// SetRealRegistryBuilder replaces the default nil builder. Called by
// internal/loop at init time. Returns the previous builder so tests
// can swap and restore.
func SetRealRegistryBuilder(b func(string) *Registry) func(string) *Registry {
	prev := realRegistry
	realRegistry = b
	return prev
}

// RealRegistry returns the registry with the concrete tools available
// in the current release. See real_registry.go in internal/loop for
// the builder. Returns nil if SetRealRegistryBuilder has not been
// called (e.g. in tests that don't import internal/loop).
func RealRegistry(projectDir string) *Registry {
	return realRegistry(projectDir)
}