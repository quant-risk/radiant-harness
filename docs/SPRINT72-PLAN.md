# Sprint 72 — MCP Tool-Bridge Adapter (v2.41.0)

> **Status**: In progress
> **Branch**: `feature/light-full-release` (continuation)
> **Target version**: v2.41.0
> **Estimated scope**: 1 sprint focado

---

## Motivation

Sprints 69-71 closed the trio of structured tools (`write_file`,
`read_file`, `search_code`, `run_gate`). These are local tools —
they operate on the project directory the harness is running in.

But MCP (Model Context Protocol) is the de facto standard for tool
distribution across the AI tooling ecosystem. Servers like
`@modelcontextprotocol/server-filesystem`,
`@modelcontextprotocol/server-github`, `mcp-server-postgres` expose
hundreds of tools the LLM can invoke. The radiant harness should
be able to **bridge** any of these servers' tools into its local
`tools.Registry` without code changes.

After Sprint 72, an operator can write:

```bash
radiant run specs/0001-foo \
  --mcp-bridge "filesystem:npx -y @modelcontextprotocol/server-filesystem ." \
  --mcp-bridge "github:npx -y @modelcontextprotocol/server-github"
```

and the LLM will see all those servers' tools alongside the local
four — same `tool_call` dispatch path, same verifier trace.

---

## Goals

| # | Goal | Acceptance |
|---|------|------------|
| G1 | MCP stdio client can `initialize`, `tools/list`, `tools/call` against any MCP server | Happy test with a mock MCP server in Go |
| G2 | MCP tools are converted to `tools.Tool` with correct `Params` from JSON Schema | Test: tool with `{name, description, inputSchema}` produces `tools.Tool` with matching Params |
| G3 | MCP tools invoke through the local `Registry.Call` path | Test: call MCP-bridged tool → MCP server receives `tools/call` JSON-RPC |
| G4 | MCP bridge accepts a list of `(name, command, args)` server specs | Multi-server test: 2 bridges registered, both discoverable |
| G5 | CLI flag `--mcp-bridge "name:command args..."` repeated | Operator can register multiple bridges from the CLI |
| G6 | Config file `.radiant.yaml` accepts `mcp_bridges: [...]` | YAML deserialises correctly |
| G7 | Failures (server won't start, non-JSON-RPC response, timeout) surface as structured errors | Error tests cover each failure mode |
| G8 | Verifier trace shows MCP-bridged tool calls with `source: mcp:<name>` | Annotate carries source info |
| G9 | Graceful shutdown: bridges closed when registry disposes | Test: defer / explicit Close cleans up subprocesses |

### Out of scope (carried to Sprint 73+)

- Anthropic/OpenAI/Gemini native function-call parsing (replace
  the markdown `tool_call` fence)
- Tool-call replay in `radiant loop export`
- HTTP/SSE transport for MCP (stdio only in this sprint)
- MCP sampling/cancellation protocol details

---

## Design

### MCP stdio client

```go
// internal/mcpbridge/client.go

type Client struct {
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout io.ReadCloser
    nextID atomic.Int64
    mu     sync.Mutex // serialises JSON-RPC writes
}

// Dial spawns the MCP server subprocess and performs the
// `initialize` handshake. Returns a client ready for tools/list
// and tools/call.
func Dial(ctx context.Context, name, command string, args []string) (*Client, error)

// ListTools returns the tools advertised by the server.
func (c *Client) ListTools(ctx context.Context) ([]MCPTool, error)

// CallTool invokes a tool by name with the given arguments.
// Returns the raw result content.
func (c *Client) CallTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error)

// Close terminates the subprocess gracefully (SIGTERM, then SIGKILL
// after a grace period).
func (c *Client) Close() error
```

### MCP tool → tools.Tool conversion

```go
// internal/mcpbridge/registry.go

type MCPTool struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    InputSchema json.RawMessage `json:"inputSchema"`
}

// ToLocalTool converts an MCP tool into a tools.Tool bound to the
// given client. The Invoke function dispatches through Client.CallTool.
func (m MCPTool) ToLocalTool(client *Client) *tools.Tool
```

The conversion flattens MCP's `inputSchema` (JSON Schema object) into
the simpler `tools.Param` slice the local registry uses. JSON Schema
type mapping:

| JSON Schema | tools.Param.Type |
|-------------|------------------|
| `"string"` | `"string"` |
| `"number"` / `"integer"` | `"number"` / `"integer"` |
| `"boolean"` | `"boolean"` |
| `"array"` | `"array"` |
| `"object"` | `"object"` |
| (other) | `"string"` (fallback) |

Required fields from `inputSchema.required` are propagated.

### CLI integration

```go
// cmd/radiant/cmd_run.go

var runMCPBridges []string // --mcp-bridge flag, repeatable

runCmd.Flags().StringArrayVar(&runMCPBridges, "mcp-bridge", nil,
    "Register an MCP server as a tool source (format: \"name:command args...\"). Repeatable.")
```

Wire-up in the engine boot:

```go
for _, spec := range runMCPBridges {
    name, command, args := parseMCPSpec(spec)
    client, err := mcpbridge.Dial(ctx, name, command, args)
    if err != nil { return err }
    tools, err := client.ListTools(ctx)
    for _, t := range tools {
        registry.Register(t.ToLocalTool(client))
    }
}
```

### Failure handling

| Failure | Behaviour |
|---------|-----------|
| Server subprocess won't start | Error before any tool call; CLI exit code 1 |
| Server returns non-JSON-RPC on stdout | Log a warning, continue; tools/list may return empty |
| `tools/call` returns error result | Surface as `mcp:<name>: <error>` in the trace |
| Server subprocess dies mid-call | Tool call returns error; bridge is marked unhealthy |
| Timeout (default 30s) | Structured error: `mcp_bridge: timeout calling <tool>` |

---

## Files

| File | Change | LOC est. |
|------|--------|----------|
| `docs/SPRINT72-PLAN.md` | NEW — this file | 200 |
| `internal/mcpbridge/client.go` | NEW — MCP stdio client | 220 |
| `internal/mcpbridge/registry.go` | NEW — MCP → tools.Tool conversion | 100 |
| `internal/mcpbridge/bridge.go` | NEW — public Dial helper + parseMCPSpec | 60 |
| `internal/mcpbridge/client_test.go` | NEW — tests with mock MCP server | 280 |
| `internal/loop/real_registry.go` | MODIFY — accept optional MCP bridges | +15 |
| `cmd/radiant/cmd_run.go` | MODIFY — `--mcp-bridge` flag + wire-up | +30 |
| `docs/TOOL-USE.md` | MODIFY — add MCP bridge section | +80 |
| `CHANGELOG.md` | MODIFY — v2.41.0 entry | +60 |
| `RELEASE-NOTES.md` | MODIFY — v2.41.0 entry | +50 |
| `cmd/radiant/main.go` | MODIFY — version bump | +1 |

**Total estimate: ~1,100 LOC** (660 new in `internal/`, ~280 tests,
~250 docs/misc).

---

## Test matrix

### Client

| # | Test | Asserts |
|---|------|---------|
| 1 | `TestClient_Initialize` | Successful handshake with mock server |
| 2 | `TestClient_ListTools` | Returns parsed tools |
| 3 | `TestClient_CallTool` | Successful tool call round-trip |
| 4 | `TestClient_CallToolError` | Server returns isError=true → error |
| 5 | `TestClient_Timeout` | Call exceeds timeout → structured error |
| 6 | `TestClient_ServerCrash` | Server dies mid-call → error |
| 7 | `TestClient_Close` | Subprocess killed cleanly |

### Registry

| # | Test | Asserts |
|---|------|---------|
| 1 | `TestMCPTool_ToLocalTool_StringParam` | JSON Schema string → tools.Param |
| 2 | `TestMCPTool_ToLocalTool_NumberParam` | JSON Schema number/integer → tools.Param |
| 3 | `TestMCPTool_ToLocalTool_Required` | required fields propagated |
| 4 | `TestMCPTool_ToLocalTool_Invoke` | Calling Invoke dispatches to Client.CallTool |

### Integration

| # | Test | Asserts |
|---|------|---------|
| 1 | `TestBridge_FullRoundtrip` | Mock server → bridge → Registry → invoke → response |
| 2 | `TestBridge_MultiServer` | 2 bridges registered, both discoverable |

---

## Risks

| Risk | Mitigation |
|------|------------|
| MCP server hangs on `initialize` | 10-second init timeout; structured error if exceeded |
| LLM calls MCP tool with bad args | Server's `isError=true` response surfaces to trace |
| Subprocess leaks if bridge not closed | `defer c.Close()` in Dial; test asserts no zombie processes |
| JSON Schema mapping incomplete (e.g. `oneOf`, `$ref`) | Pass-through fallback: emit raw `inputSchema` as `params` metadata for the LLM to inspect |
| Operator misspells --mcp-bridge spec | Parse error before subprocess spawn; clear error message |

---

## Commit plan

Single commit on `feature/light-full-release`:

```
feat(mcp-bridge): Sprint 72 — MCP tool bridge adapter (v2.41.0)
```

Pass criteria: `go vet ./...` clean, `go test -count=1 -v ./...`
green (981+ tests), cross-compile 3/3 platforms.

---

**Status at plan write**: Sprint 71 (v2.40.0) committed at `a033803`.
Sprint 72 implementation in progress.